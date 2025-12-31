package artifactd

const (
	ddlCommit = `
CREATE TABLE IF NOT EXISTS commit
(
    namespace String,
    name String,
	id UInt32,
    commit_id String,
	os_arch Array(String),
	published_at DateTime,
    branch String,
    tag String,
	actor String,
	data String,
	source String,
)
ENGINE = MergeTree()
PRIMARY KEY (namespace, name, id)
`

	// index by commit
	dmlRegisterCommit = `INSERT INTO commit (namespace, name, id, commit_id, branch, tag, actor, data, published_at, source, os_arch) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`

	// get commit
	dqlGetCommit = `SELECT namespace, id, name, commit_id, branch, tag, actor, data, published_at, source, os_arch FROM commit WHERE namespace = ? and name = ? and published_at >= ? ORDER BY id desc;`
)
