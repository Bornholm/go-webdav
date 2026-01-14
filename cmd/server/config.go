package main

import (
	"encoding"
	"encoding/json"
)

type config struct {
	Auth       authConfig       `json:"auth" envPrefix:"AUTH_"`
	Filesystem filesystemConfig `json:"filesystem" envPrefix:"FILESYSTEM_"`
}

type authConfig struct {
	Enabled bool              `json:"enabled" env:"ENABLED" envDefault:"true"`
	Users   map[string]string `json:"users" env:"USERS,expand"`
}

type filesystemConfig struct {
	Type    string   `json:"type" env:"TYPE,expand"`
	Options *rawJSON `json:"options" env:"OPTIONS,expand"`
}

type rawJSON struct {
	Value any
}

// UnmarshalJSON implements [json.Unmarshaler].
func (j *rawJSON) UnmarshalJSON(data []byte) error {
	if err := json.Unmarshal(data, &j.Value); err != nil {
		return err
	}

	return nil
}

// UnmarshalText implements [encoding.TextUnmarshaler].
func (j *rawJSON) UnmarshalText(text []byte) error {
	if err := json.Unmarshal(text, &j.Value); err != nil {
		return err
	}

	return nil
}

var _ encoding.TextUnmarshaler = &rawJSON{}
var _ json.Unmarshaler = &rawJSON{}
