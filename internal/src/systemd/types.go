package systemd

type Row[T any] struct {
	Name string `json:"name"`
	Key  string `json:"key"`
	Data T      `json:"data"`
}

type DBusUnitStatus struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	LoadState   string `json:"load_state"`
	ActiveState string `json:"active_state"`
	SubState    string `json:"sub_state"`
	Followed    string `json:"followed"`
	Path        string `json:"path"`
	JobId       uint32 `json:"job_id"`
	JobType     string `json:"job_type"`
	JobPath     string `json:"job_path"`
}
