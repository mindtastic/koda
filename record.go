package koda

import "errors"

// AccountKey is the primary identifier for a User. It is Service agnostic.
type AccountKey string

// ServiceKey is a service dependent primary key for a User.
type ServiceKey string

type Record struct {
	AccountKey AccountKey `json:"accountKey"`
	Inactive   bool       `json:"inactive"`

	// ServiceKeys maps the name of a service to a ServiceKey for the specific user.
	ServiceKeys map[string]ServiceKey `json:"serviceKeys"`
}

var ErrNotFound = errors.New("not found")

// A Store must be able to store and retrieve records based on a given AccountKey only.
// ServiceKeys must not be used for indexing.
type Store interface {
	Set(AccountKey, Record) error
	Get(key AccountKey) (Record, error)
}
