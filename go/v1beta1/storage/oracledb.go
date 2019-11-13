package storage

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	"database/sql"

	"github.com/fernet/fernet-go"
	grafeasConfig "github.com/grafeas/grafeas/go/config"
	"github.com/grafeas/grafeas/go/name"
	"github.com/grafeas/grafeas/go/v1beta1/storage"
	pb "github.com/grafeas/grafeas/proto/v1beta1/grafeas_go_proto"
	prpb "github.com/grafeas/grafeas/proto/v1beta1/project_go_proto"
	"github.com/judavi/grafeas-oracle/go/config"
	"github.com/lib/pq"
	fieldmaskpb "google.golang.org/genproto/protobuf/field_mask"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	goracle "gopkg.in/goracle.v2"
)

type OracleDb struct {
	*sql.DB
	paginationKey string
}

func OracleStorageTypeProvider(storageType string, storageConfig *grafeasConfig.StorageConfiguration) (*storage.Storage, error) {
	if storageType != "oracle" {
		return nil, errors.New(fmt.Sprintf("Unknown storage type %s, must be 'oracle'", storageType))
	}

	var storeConfig config.OracleConfig

	err := grafeasConfig.ConvertGenericConfigToSpecificType(storageConfig, &storeConfig)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Unable to create OracleConfig, %s", err))
	}

	s := NewOracleStore(&storeConfig)
	storage := &storage.Storage{
		Ps: s,
		Gs: s,
	}

	return storage, nil
}

func NewOracleStore(config *config.OracleConfig) *OracleDb {
	/*
		err := createDatabase(CreateSourceString(config.User, config.Password, config.Host, config.DbName), config.DbName)
		if err != nil {
			log.Fatal(err.Error())
		}*/
	db, err := sql.Open("goracle", CreateSourceString(config.User, config.Password, config.Host, config.DbName))
	if err != nil {
		log.Fatal(err.Error())
	}
	var serverVersion goracle.VersionInfo
	serverVersion, err = goracle.ServerVersion(context.TODO(), db)
	log.Println(serverVersion)
	if db.Ping() != nil {
		log.Fatal("Database server is not alive")
	}
	log.Println("Before create tables")
	/*
		if _, err := db.Exec(createTables); err != nil {
			log.Println("Fatal Error")
			db.Close()
			log.Fatal(err.Error())
		}
	*/
	log.Println("After create tables")
	return &OracleDb{
		DB:            db,
		paginationKey: config.PaginationKey,
	}
}

func createDatabase(source, dbName string) error {
	db, err := sql.Open("goracle", source)
	if err != nil {
		return err
	}
	defer db.Close()
	log.Println("Check if db exists")
	// Check if db exists
	res, err := db.Exec(
		fmt.Sprintf("SELECT USERNAME FROM ALL_USERS WHERE USERNAME = '%s'", dbName))
	if err != nil {
		log.Println(err)
		return err
	}
	rowCnt, err := res.RowsAffected()
	if err != nil {
		return err
	}
	// Create database if it doesn't exist
	if rowCnt == 0 {
		/*
			_, err = db.Exec(fmt.Sprintf("CREATE DATABASE %s", dbName))
			if err != nil {
				return err
			}
		*/
	}
	return nil
}

// CreateProject adds the specified project to the store
func (pg *OracleDb) CreateProject(ctx context.Context, pID string, p *prpb.Project) (*prpb.Project, error) {
	_, err := pg.DB.Exec(insertProject, name.FormatProject(pID))
	if err, ok := err.(*pq.Error); ok {
		// Check for unique_violation
		if err.Code == "23505" {
			return nil, status.Errorf(codes.AlreadyExists, "Project with name %q already exists", pID)
		} else {
			log.Println("Failed to insert Project in database", err)
			return nil, status.Error(codes.Internal, "Failed to insert Project in database")
		}
	}
	return p, nil
}

// DeleteProject deletes the project with the given pID from the store
func (pg *OracleDb) DeleteProject(ctx context.Context, pID string) error {

	pName := name.FormatProject(pID)
	result, err := pg.DB.Exec(deleteProject, pName)
	if err != nil {
		return status.Error(codes.Internal, "Failed to delete Project from database")
	}
	count, err := result.RowsAffected()
	if err != nil {
		return status.Error(codes.Internal, "Failed to delete Project from database")
	}
	if count == 0 {
		return status.Errorf(codes.NotFound, "Project with name %q does not Exist", pName)
	}
	return nil
}

// GetProject returns the project with the given pID from the store
func (pg *OracleDb) GetProject(ctx context.Context, pID string) (*prpb.Project, error) {

	pName := name.FormatProject(pID)
	var exists bool
	err := pg.DB.QueryRow(projectExists, pName).Scan(&exists)
	if err != nil {
		return nil, status.Error(codes.Internal, "Failed to query Project from database")
	}
	if !exists {
		return nil, status.Errorf(codes.NotFound, "Project with name %q does not Exist", pName)
	}
	return &prpb.Project{Name: pName}, nil
}

// ListProjects returns up to pageSize number of projects beginning at pageToken (or from
// start if pageToken is the empty string).
func (pg *OracleDb) ListProjects(ctx context.Context, filter string, pageSize int, pageToken string) ([]*prpb.Project, string, error) {
	var rows *sql.Rows
	id := decryptInt64(pageToken, pg.paginationKey, 0)
	rows, err := pg.DB.Query(listProjects, id, pageSize)
	if err != nil {
		return nil, "", status.Error(codes.Internal, "Failed to list Projects from database")
	}
	count, err := pg.count(projectCount)
	if err != nil {
		return nil, "", status.Error(codes.Internal, "Failed to count Projects from database")
	}
	var projects []*prpb.Project
	var lastId int64
	for rows.Next() {
		var name string
		err := rows.Scan(&lastId, &name)
		if err != nil {
			return nil, "", status.Error(codes.Internal, "Failed to scan Project row")
		}
		projects = append(projects, &prpb.Project{Name: name})
	}
	if count == lastId {
		return projects, "", nil
	}
	encryptedPage, err := encryptInt64(lastId, pg.paginationKey)
	if err != nil {
		return nil, "", status.Error(codes.Internal, "Failed to paginate projects")
	}
	return projects, encryptedPage, nil
}

// CreateNote adds the specified note
func (pg *OracleDb) CreateNote(ctx context.Context, pID, nID, uID string, n *pb.Note) (*pb.Note, error) {
	return nil, nil
}

// BatchCreateNotes batch creates the specified notes in memstore.
func (pg *OracleDb) BatchCreateNotes(ctx context.Context, pID, uID string, notes map[string]*pb.Note) ([]*pb.Note, []error) {
	return nil, nil
}

// DeleteNote deletes the note with the given pID and nID
func (pg *OracleDb) DeleteNote(ctx context.Context, pID, nID string) error {
	return nil
}

// UpdateNote updates the existing note with the given pID and nID
func (pg *OracleDb) UpdateNote(ctx context.Context, pID, nID string, n *pb.Note, mask *fieldmaskpb.FieldMask) (*pb.Note, error) {
	return nil, nil
}

// GetNote returns the note with project (pID) and note ID (nID)
func (pg *OracleDb) GetNote(ctx context.Context, pID, nID string) (*pb.Note, error) {
	return nil, nil
}

// CreateOccurrence adds the specified occurrence
func (pg *OracleDb) CreateOccurrence(ctx context.Context, pID, uID string, o *pb.Occurrence) (*pb.Occurrence, error) {

	return o, nil
}

// BatchCreateOccurrence batch creates the specified occurrences in PostreSQL.
func (pg *OracleDb) BatchCreateOccurrences(ctx context.Context, pID string, uID string, occs []*pb.Occurrence) ([]*pb.Occurrence, []error) {
	return nil, nil
}

// DeleteOccurrence deletes the occurrence with the given pID and oID
func (pg *OracleDb) DeleteOccurrence(ctx context.Context, pID, oID string) error {
	return nil
}

// UpdateOccurrence updates the existing occurrence with the given projectID and occurrenceID
func (pg *OracleDb) UpdateOccurrence(ctx context.Context, pID, oID string, o *pb.Occurrence, mask *fieldmaskpb.FieldMask) (*pb.Occurrence, error) {
	return nil, nil
}

// GetOccurrence returns the occurrence with pID and oID
func (pg *OracleDb) GetOccurrence(ctx context.Context, pID, oID string) (*pb.Occurrence, error) {
	return nil, nil
}

// ListOccurrences returns up to pageSize number of occurrences for this project beginning
// at pageToken, or from start if pageToken is the empty string.
func (pg *OracleDb) ListOccurrences(ctx context.Context, pID, filter, pageToken string, pageSize int32) ([]*pb.Occurrence, string, error) {
	return nil, "", nil
}

// GetOccurrenceNote gets the note for the specified occurrence from PostgreSQL.
func (pg *OracleDb) GetOccurrenceNote(ctx context.Context, pID, oID string) (*pb.Note, error) {
	return nil, nil
}

// ListNotes returns up to pageSize number of notes for this project (pID) beginning
// at pageToken (or from start if pageToken is the empty string).
func (pg *OracleDb) ListNotes(ctx context.Context, pID, filter, pageToken string, pageSize int32) ([]*pb.Note, string, error) {
	return nil, "", nil
}

// ListNoteOccurrences returns up to pageSize number of occcurrences on the particular note (nID)
// for this project (pID) projects beginning at pageToken (or from start if pageToken is the empty string).
func (pg *OracleDb) ListNoteOccurrences(ctx context.Context, pID, nID, filter, pageToken string, pageSize int32) ([]*pb.Occurrence, string, error) {
	return nil, "", nil
}

// GetVulnerabilityOccurrencesSummary gets a summary of vulnerability occurrences from storage.
func (pg *OracleDb) GetVulnerabilityOccurrencesSummary(ctx context.Context, projectID, filter string) (*pb.VulnerabilityOccurrencesSummary, error) {
	return &pb.VulnerabilityOccurrencesSummary{}, nil
}

// count returns the total number of entries for the specified query (assuming SELECT(*) is used)
func (pg *OracleDb) count(query string, args ...interface{}) (int64, error) {
	row := pg.DB.QueryRow(query, args...)
	var count int64
	err := row.Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, err
}

// Encrypt int64 using provided key
func encryptInt64(v int64, key string) (string, error) {
	k, err := fernet.DecodeKey(key)
	if err != nil {
		return "", err
	}
	bytes, err := fernet.EncryptAndSign([]byte(strconv.FormatInt(v, 10)), k)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// Decrypts encrypted int64 using provided key. Returns defaultValue if decryption fails.
func decryptInt64(encrypted string, key string, defaultValue int64) int64 {
	k, err := fernet.DecodeKey(key)
	if err != nil {
		return defaultValue
	}
	bytes := fernet.VerifyAndDecrypt([]byte(encrypted), time.Hour, []*fernet.Key{k})
	if bytes == nil {
		return defaultValue
	}
	decryptedValue, err := strconv.ParseInt(string(bytes), 10, 64)
	if err != nil {
		return defaultValue
	}
	return decryptedValue
}

// CreateSourceString generates DB source path.
func CreateSourceString(user string, password string, host string, dbName string) string {
	log.Println(fmt.Sprintf("%s/%s@%s/%s", user, password, host, dbName))
	return fmt.Sprintf("%s/%s@%s/%s", user, password, host, dbName)
}
