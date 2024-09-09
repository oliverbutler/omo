package utils

import "time"

type IsoDate string

func (d IsoDate) String() string {
	return string(d)
}

func (d IsoDate) After(other IsoDate) bool {
	return d > other
}

func (d IsoDate) FormattedString() string {
	t, err := time.Parse("2006-01-02", string(d))
	if err != nil {
		return string(d) // Return original string if parsing fails
	}
	return t.Format("January 2, 2006")
}
