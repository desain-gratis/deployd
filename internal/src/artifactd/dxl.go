package artifactd

const (
	// index by commit
	dmlRegisterCommit = `INSERT INTO commit (namespace, name, commit_id, branch, tag, actor, data, published_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?);`

	// get commit
	dqlGetCommit = `SELECT namespace, name, commit_id, branch, tag, actor, data, published_at FROM commit WHERE namespace = ? and name = ? and published_at >= ? ;`
)
