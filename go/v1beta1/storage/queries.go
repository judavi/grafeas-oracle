package storage

const (
	createTableProjects = `
			CREATE TABLE projects (
				id NUMBER(10) PRIMARY KEY,
				name VARCHAR2(3000) NOT NULL UNIQUE
			)
		`
	createSequenceProjects = `CREATE SEQUENCE projects_seq START WITH 1 INCREMENT BY 1`
	createTriggerProjects  = `		
			CREATE OR REPLACE TRIGGER projects_seq_tr
				BEFORE INSERT ON projects FOR EACH ROW
				WHEN (NEW.id IS NULL)
				BEGIN
					SELECT projects_seq.NEXTVAL INTO :NEW.id FROM DUAL;
				END;`
	createTableNotes = `
			CREATE TABLE notes (
				id NUMBER(10) PRIMARY KEY,
				project_name VARCHAR2(3000) NOT NULL,
				note_name VARCHAR2(3000) NOT NULL,
				data CLOB,
				UNIQUE (project_name, note_name),
				CONSTRAINT notes_json CHECK (data IS JSON)
			)
		`
	createSequenceNotes = `CREATE SEQUENCE notes_seq START WITH 1 INCREMENT BY 1`
	createTriggerNotes  = `		
		CREATE OR REPLACE TRIGGER notes_seq_tr
			BEFORE INSERT ON notes FOR EACH ROW
			WHEN (NEW.id IS NULL)
			BEGIN
				SELECT notes_seq.NEXTVAL INTO :NEW.id FROM DUAL;
			END;`

	createTableOccurrences = `
			CREATE TABLE occurrences (
				id NUMBER(10) PRIMARY KEY,
				project_name VARCHAR2(3000) NOT NULL,
				occurrence_name VARCHAR2(3000) NOT NULL,
				data CLOB,
				note_id number(10) REFERENCES notes NOT NULL,
				UNIQUE (project_name, occurrence_name),
				CONSTRAINT occurrences_json CHECK (data IS JSON)
			)
		`
	createSequenceOccurrences = `CREATE SEQUENCE occurrences_seq START WITH 1 INCREMENT BY 1`
	createTriggerOccurrences  = `
		CREATE OR REPLACE TRIGGER occurrences_seq_tr
			BEFORE INSERT ON occurrences FOR EACH ROW
			WHEN (NEW.id IS NULL)
			BEGIN
				SELECT occurrences_seq.NEXTVAL INTO :NEW.id FROM DUAL;
			END;
		`

	checkIfTablesExists = `SELECT count(*) as count FROM SYS.dba_tables where table_name IN ('PROJECTS', 'NOTES', 'OCCURRENCES', 'OPERATIONS')`

	insertProject = `INSERT INTO projects(name) VALUES (:1)`
	projectExists = `SELECT count(*) as "exists" FROM projects WHERE name = :1`
	deleteProject = `DELETE FROM projects WHERE name = :1`
	listProjects  = `SELECT ROW_NUMBER() OVER (ORDER BY id), name FROM projects ORDER BY id OFFSET :2 ROWS FETCH FIRST :3 ROWS ONLY`
	projectCount  = `SELECT COUNT(*) FROM projects`

	insertOccurrence = `INSERT INTO occurrences(project_name, occurrence_name, note_id, data)
                      VALUES (:1, :2, (SELECT id FROM notes WHERE project_name = :3 AND note_name = :4), :5)`
	searchOccurrence        = `SELECT data FROM occurrences WHERE project_name = :1 AND occurrence_name = :2`
	updateOccurrence        = `UPDATE occurrences SET data = :1 WHERE project_name = :2 AND occurrence_name = :3`
	deleteOccurrence        = `DELETE FROM occurrences WHERE project_name = :1 AND occurrence_name = :2`
	listOccurrences         = `SELECT ROW_NUMBER() OVER (ORDER BY id), data FROM occurrences WHERE project_name = :1 ORDER BY id OFFSET :3 ROWS FETCH FIRST :4 ROWS ONLY`
	listOccurrencesFiltered = `SELECT id, data FROM (SELECT
																ROW_NUMBER() OVER (ORDER BY id) AS id,
																o.data AS data
															FROM occurrences o
															WHERE
															  project_name = :1
																AND o.data.kind = :2
																AND o.data."resource".uri = :3
															ORDER BY id OFFSET :4 ROWS FETCH FIRST :5 ROWS ONLY)`
	occurrenceCount         = `SELECT COUNT(*) FROM occurrences WHERE project_name = :1`
	occurrenceCountFiltered = `SELECT COUNT(*)
															FROM occurrences o
															WHERE
															  project_name = :1
																AND o.data.kind = :2
																AND o.data."resource".uri = :3`

	insertNote          = `INSERT INTO notes(project_name, note_name, data) VALUES (:1, :2, :3)`
	searchNote          = `SELECT data FROM notes WHERE project_name = :1 AND note_name = :2`
	updateNote          = `UPDATE notes SET data = :1 WHERE project_name = :2 AND note_name = :3`
	deleteNote          = `DELETE FROM notes WHERE project_name = :1 AND note_name = :2`
	listNotes           = `SELECT ROW_NUMBER() OVER (ORDER BY id), data FROM notes WHERE project_name = :1 ORDER BY id OFFSET :3 ROWS FETCH FIRST :4 ROWS ONLY`
	noteCount           = `SELECT COUNT(*) FROM notes WHERE project_name = :1`
	listNoteOccurrences = `SELECT T as id, data(SELECT ROW_NUMBER() OVER (ORDER BY o.id) AS T , o.id, o.data FROM occurrences o, notes n
													WHERE n.id = o.note_id
														AND n.project_name = :1
														AND n.note_name = :2
														ORDER BY T OFFSET :3 ROWS FETCH FIRST :4 ROWS ONLY)`
	noteOccurrencesCount = `SELECT COUNT(*) FROM occurrences o, notes  n
	                         WHERE n.id = o.note_id
	                           AND n.project_name = :1
	                           AND n.note_name = :2`
)
