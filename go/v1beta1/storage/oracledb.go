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
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/google/uuid"
	grafeasConfig "github.com/grafeas/grafeas/go/config"
	"github.com/grafeas/grafeas/go/name"
	"github.com/grafeas/grafeas/go/v1beta1/storage"
	pb "github.com/grafeas/grafeas/proto/v1beta1/grafeas_go_proto"
	prpb "github.com/grafeas/grafeas/proto/v1beta1/project_go_proto"
	"github.com/judavi/grafeas-oracle/go/config"
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

	err := setupDatabase(CreateSourceString(config.User, config.Password, config.Host, config.DbName), config.DbName)
	if err != nil {
		log.Fatal(err.Error())
	}
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

	return &OracleDb{
		DB:            db,
		paginationKey: config.PaginationKey,
	}
}

func setupDatabase(source, dbName string) error {
	db, err := sql.Open("goracle", source)
	if err != nil {
		return err
	}
	defer db.Close()
	//log.Println("Check if tables exists")
	// Check if db exists
	var exists int
	err = db.QueryRow(checkIfTablesExists).Scan(&exists)
	if err != nil {
		log.Println(err)
		return err
	}
	if exists == 0 {
		creationOperations := [9]string{
			createTableProjects,
			createTableNotes,
			createTableOccurrences,
			createSequenceProjects,
			createSequenceNotes,
			createSequenceOccurrences,
			createTriggerProjects,
			createTriggerNotes,
			createTriggerOccurrences,
		}
		for _, element := range creationOperations {
			stmt, err := db.Prepare(element)
			if err != nil {
				return err
			}
			defer stmt.Close()
			_, err = stmt.Exec()
			if err != nil {
				log.Fatal(err)
			}
		}

	}
	return nil
}

// CreateProject adds the specified project to the store
func (pg *OracleDb) CreateProject(ctx context.Context, pID string, p *prpb.Project) (*prpb.Project, error) {
	_, err := pg.DB.Exec(insertProject, name.FormatProject(pID))
	if err, ok := goracle.AsOraErr(err); ok {
		// Check for unique_violation
		if err.Code() == 1 {
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
	var exists int
	err := pg.DB.QueryRow(projectExists, pName).Scan(&exists)
	if err != nil {
		return nil, status.Error(codes.Internal, "Failed to query Project from database")
	}
	if exists == 0 {
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
		log.Println(err)
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

	return nil, encryptedPage, nil
}

// CreateNote adds the specified note
func (pg *OracleDb) CreateNote(ctx context.Context, pID, nID, uID string, n *pb.Note) (*pb.Note, error) {
	n = proto.Clone(n).(*pb.Note)
	nName := name.FormatNote(pID, nID)
	n.Name = nName
	n.CreateTime = ptypes.TimestampNow()

	_, err := pg.DB.Exec(insertNote, pID, nID, proto.MarshalTextString(n))
	if err, ok := goracle.AsOraErr(err); ok {
		// Check for unique_violation
		if err.Code() == 1 {
			return nil, status.Errorf(codes.AlreadyExists, "Note with name %q already exists", n.Name)
		} else {
			log.Println("Failed to insert Note in database", err)
			return nil, status.Error(codes.Internal, "Failed to insert Note in database")
		}
	}
	return n, nil
}

// BatchCreateNotes batch creates the specified notes in memstore.
func (pg *OracleDb) BatchCreateNotes(ctx context.Context, pID, uID string, notes map[string]*pb.Note) ([]*pb.Note, []error) {
	clonedNotes := map[string]*pb.Note{}
	for nID, n := range notes {
		clonedNotes[nID] = proto.Clone(n).(*pb.Note)
	}
	notes = clonedNotes

	errs := []error{}
	created := []*pb.Note{}
	for nID, n := range notes {
		note, err := pg.CreateNote(ctx, pID, nID, uID, n)
		if err != nil {
			// Note already exists, skipping.
			continue
		} else {
			created = append(created, note)
		}

	}

	return created, errs
}

// DeleteNote deletes the note with the given pID and nID
func (pg *OracleDb) DeleteNote(ctx context.Context, pID, nID string) error {
	result, err := pg.DB.Exec(deleteNote, pID, nID)
	if err != nil {
		return status.Error(codes.Internal, "Failed to delete Note from database")
	}
	count, err := result.RowsAffected()
	if err != nil {
		return status.Error(codes.Internal, "Failed to delete Note from database")
	}
	if count == 0 {
		return status.Errorf(codes.NotFound, "Note with name %q/%q does not Exist", pID, nID)
	}
	return nil
}

// UpdateNote updates the existing note with the given pID and nID
func (pg *OracleDb) UpdateNote(ctx context.Context, pID, nID string, n *pb.Note, mask *fieldmaskpb.FieldMask) (*pb.Note, error) {
	n = proto.Clone(n).(*pb.Note)
	nName := name.FormatNote(pID, nID)
	n.Name = nName
	// TODO(#312): implement the update operation
	n.UpdateTime = ptypes.TimestampNow()

	result, err := pg.DB.Exec(updateNote, proto.MarshalTextString(n), pID, nID)
	if err != nil {
		return nil, status.Error(codes.Internal, "Failed to update Note")
	}
	count, err := result.RowsAffected()
	if err != nil {
		return nil, status.Error(codes.Internal, "Failed to update Note")
	}
	if count == 0 {
		return nil, status.Errorf(codes.NotFound, "Note with name %q/%q does not Exist", pID, nID)
	}
	return n, nil
}

// GetNote returns the note with project (pID) and note ID (nID)
func (pg *OracleDb) GetNote(ctx context.Context, pID, nID string) (*pb.Note, error) {
	var data string
	err := pg.DB.QueryRow(searchNote, pID, nID).Scan(&data)
	switch {
	case err == sql.ErrNoRows:
		return nil, status.Errorf(codes.NotFound, "Note with name %q/%q does not Exist", pID, nID)
	case err != nil:
		return nil, status.Error(codes.Internal, "Failed to query Note from database")
	}
	var note pb.Note
	proto.UnmarshalText(data, &note)
	if err != nil {
		return nil, status.Error(codes.Internal, "Failed to unmarshal Note from database")
	}
	// Set the output-only field before returning
	note.Name = name.FormatNote(pID, nID)
	return &note, nil
}

// CreateOccurrence adds the specified occurrence
func (pg *OracleDb) CreateOccurrence(ctx context.Context, pID, uID string, o *pb.Occurrence) (*pb.Occurrence, error) {
	o = proto.Clone(o).(*pb.Occurrence)
	o.CreateTime = ptypes.TimestampNow()

	var id string
	if nr, err := uuid.NewRandom(); err != nil {
		return nil, status.Error(codes.Internal, "Failed to generate UUID")
	} else {
		id = nr.String()
	}
	o.Name = fmt.Sprintf("projects/%s/occurrences/%s", pID, id)

	nPID, nID, err := name.ParseNote(o.NoteName)
	if err != nil {
		log.Printf("Invalid note name: %v", o.NoteName)
		return nil, status.Error(codes.InvalidArgument, "Invalid note name")
	}
	_, err = pg.DB.Exec(insertOccurrence, pID, id, nPID, nID, proto.MarshalTextString(o))
	if err, ok := goracle.AsOraErr(err); ok {
		// Check for unique_violation
		//This will never happen because the occurrence name is unique
		if err.Code() == 1 {
			return nil, status.Errorf(codes.AlreadyExists, "Occurrence with name %q already exists", o.Name)
		} else {
			log.Println("Failed to insert Occurrence in database", err)
			return nil, status.Error(codes.Internal, "Failed to insert Occurrence in database")
		}
	}
	return o, nil
}

// BatchCreateOccurrence batch creates the specified occurrences in PostreSQL.
func (pg *OracleDb) BatchCreateOccurrences(ctx context.Context, pID string, uID string, occs []*pb.Occurrence) ([]*pb.Occurrence, []error) {
	clonedOccs := []*pb.Occurrence{}
	for _, o := range occs {
		clonedOccs = append(clonedOccs, proto.Clone(o).(*pb.Occurrence))
	}
	occs = clonedOccs

	errs := []error{}
	created := []*pb.Occurrence{}
	for _, o := range occs {
		occ, err := pg.CreateOccurrence(ctx, pID, uID, o)
		if err != nil {
			// Occurrence already exists, skipping.
			continue
		} else {
			created = append(created, occ)
		}
	}

	return created, errs
}

//TODO
// DeleteOccurrence deletes the occurrence with the given pID and oID
func (pg *OracleDb) DeleteOccurrence(ctx context.Context, pID, oID string) error {
	result, err := pg.DB.Exec(deleteOccurrence, pID, oID)
	if err != nil {
		return status.Error(codes.Internal, "Failed to delete Occurrence from database")
	}
	count, err := result.RowsAffected()
	if err != nil {
		return status.Error(codes.Internal, "Failed to delete Occurrence from database")
	}
	if count == 0 {
		return status.Errorf(codes.NotFound, "Occurrence with name %q/%q does not Exist", pID, oID)
	}
	return nil
}

// UpdateOccurrence updates the existing occurrence with the given projectID and occurrenceID
func (pg *OracleDb) UpdateOccurrence(ctx context.Context, pID, oID string, o *pb.Occurrence, mask *fieldmaskpb.FieldMask) (*pb.Occurrence, error) {
	o = proto.Clone(o).(*pb.Occurrence)
	// TODO(#312): implement the update operation
	o.UpdateTime = ptypes.TimestampNow()

	result, err := pg.DB.Exec(updateOccurrence, proto.MarshalTextString(o), pID, oID)
	if err != nil {
		return nil, status.Error(codes.Internal, "Failed to update Occurrence")
	}
	count, err := result.RowsAffected()
	if err != nil {
		return nil, status.Error(codes.Internal, "Failed to update Occurrence")
	}
	if count == 0 {
		return nil, status.Errorf(codes.NotFound, "Occurrence with name %q/%q does not Exist", pID, oID)
	}
	return o, nil
}

// GetOccurrence returns the occurrence with pID and oID
func (pg *OracleDb) GetOccurrence(ctx context.Context, pID, oID string) (*pb.Occurrence, error) {
	var data string
	err := pg.DB.QueryRow(searchOccurrence, pID, oID).Scan(&data)
	switch {
	case err == sql.ErrNoRows:
		return nil, status.Errorf(codes.NotFound, "Occurrence with name %q/%q does not Exist", pID, oID)
	case err != nil:
		return nil, status.Error(codes.Internal, "Failed to query Occurrence from database")
	}
	var o pb.Occurrence
	proto.UnmarshalText(data, &o)
	if err != nil {
		return nil, status.Error(codes.Internal, "Failed to unmarshal Occurrence from database")
	}
	// Set the output-only field before returning
	o.Name = name.FormatOccurrence(pID, oID)
	return &o, nil
}

// ListOccurrences returns up to pageSize number of occurrences for this project beginning
// at pageToken, or from start if pageToken is the empty string.
func (pg *OracleDb) ListOccurrences(ctx context.Context, pID, filter, pageToken string, pageSize int32) ([]*pb.Occurrence, string, error) {
	var rows *sql.Rows
	id := decryptInt64(pageToken, pg.paginationKey, 0)
	//id := pageToken // decryptInt64(pageToken, pg.paginationKey, 0)
	//log.Print(id)
	rows, err := pg.DB.Query(listOccurrences, pID, id, pageSize)
	if err != nil {
		return nil, "", status.Error(codes.Internal, "Failed to list Occurrences from database")
	}
	count, err := pg.count(occurrenceCount, pID)
	if err != nil {
		return nil, "", status.Error(codes.Internal, "Failed to count Occurrences from database")
	}
	var os []*pb.Occurrence
	var lastId int64
	for rows.Next() {
		var data string
		err := rows.Scan(&lastId, &data)
		//log.Print(lastId)
		if err != nil {
			return nil, "", status.Error(codes.Internal, "Failed to scan Occurrences row")
		}
		var o pb.Occurrence
		proto.UnmarshalText(data, &o)
		if err != nil {
			return nil, "", status.Error(codes.Internal, "Failed to unmarshal Occurrence from database")
		}
		os = append(os, &o)
	}
	if count == lastId {
		return os, "", nil
	}

	encryptedPage, err := encryptInt64(lastId, pg.paginationKey)
	if err != nil {
		return nil, "", status.Error(codes.Internal, "Failed to paginate projects")
	}

	return os, encryptedPage, nil
	//return os, strconv.FormatInt(int64(lastId), 10), nil
}

// GetOccurrenceNote gets the note for the specified occurrence from PostgreSQL.
func (pg *OracleDb) GetOccurrenceNote(ctx context.Context, pID, oID string) (*pb.Note, error) {
	return nil, nil
}

// ListNotes returns up to pageSize number of notes for this project (pID) beginning
// at pageToken (or from start if pageToken is the empty string).
func (pg *OracleDb) ListNotes(ctx context.Context, pID, filter, pageToken string, pageSize int32) ([]*pb.Note, string, error) {
	var rows *sql.Rows
	id := decryptInt64(pageToken, pg.paginationKey, 0)
	rows, err := pg.DB.Query(listNotes, pID, id, pageSize)
	if err != nil {
		return nil, "", status.Error(codes.Internal, "Failed to list Notes from database")
	}
	count, err := pg.count(noteCount, pID)
	if err != nil {
		return nil, "", status.Error(codes.Internal, "Failed to count Notes from database")
	}
	var ns []*pb.Note
	var lastId int64
	for rows.Next() {
		var data string
		err := rows.Scan(&lastId, &data)
		if err != nil {
			return nil, "", status.Error(codes.Internal, "Failed to scan Notes row")
		}
		var n pb.Note
		proto.UnmarshalText(data, &n)
		if err != nil {
			return nil, "", status.Error(codes.Internal, "Failed to unmarshal Note from database")
		}
		ns = append(ns, &n)
	}
	if count == lastId {
		return ns, "", nil
	}
	encryptedPage, err := encryptInt64(lastId, pg.paginationKey)
	if err != nil {
		return nil, "", status.Error(codes.Internal, "Failed to paginate projects")
	}
	return ns, encryptedPage, nil
}

// ListNoteOccurrences returns up to pageSize number of occcurrences on the particular note (nID)
// for this project (pID) projects beginning at pageToken (or from start if pageToken is the empty string).
func (pg *OracleDb) ListNoteOccurrences(ctx context.Context, pID, nID, filter, pageToken string, pageSize int32) ([]*pb.Occurrence, string, error) {
	// Verify that note exists
	if _, err := pg.GetNote(ctx, pID, nID); err != nil {
		return nil, "", err
	}
	var rows *sql.Rows
	id := decryptInt64(pageToken, pg.paginationKey, 0)

	if pageSize == 0 {
		pageSize = 10
	}
	log.Print(pID)
	log.Print(nID)
	log.Print(id)
	log.Print(pageSize)
	rows, err := pg.DB.Query(listNoteOccurrences, pID, nID, id, pageSize)
	if err != nil {
		return nil, "", status.Error(codes.Internal, "Failed to list Occurrences from database")
	}

	count, err := pg.count(noteOccurrencesCount, pID, nID)
	if err != nil {
		return nil, "", status.Error(codes.Internal, "Failed to count Occurrences from database")
	}
	var os []*pb.Occurrence
	var lastId int64
	for rows.Next() {
		var data string
		err := rows.Scan(&lastId, &data)
		if err != nil {
			return nil, "", status.Error(codes.Internal, "Failed to scan Occurrences row")
		}
		var o pb.Occurrence
		proto.UnmarshalText(data, &o)
		if err != nil {
			return nil, "", status.Error(codes.Internal, "Failed to unmarshal Occurrence from database")
		}
		os = append(os, &o)
	}
	if count == lastId {
		return os, "", nil
	}
	encryptedPage, err := encryptInt64(lastId, pg.paginationKey)
	if err != nil {
		return nil, "", status.Error(codes.Internal, "Failed to paginate projects")
	}
	return os, encryptedPage, nil

}

// GetVulnerabilityOccurrencesSummary gets a summary of vulnerability occurrences from storage.
func (pg *OracleDb) GetVulnerabilityOccurrencesSummary(ctx context.Context, projectID, filter string) (*pb.VulnerabilityOccurrencesSummary, error) {
	//TODO
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
	//log.Println(fmt.Sprintf("%s/%s@%s/%s", user, password, host, dbName))
	return fmt.Sprintf("%s/%s@%s/%s", user, password, host, dbName)
}
