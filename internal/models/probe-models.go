package models

type LocationOptions struct {
	Buildings   []string `json:"buildings"`
	Floors      []string `json:"floors"`
	Rooms       []string `json:"rooms"`
	Departments []string `json:"departments"`
}
