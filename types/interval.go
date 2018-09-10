package types

import (
	"strconv"
	"time"
)

//Interval is a duration of time
type Interval time.Duration

//UnmarshalJSON unmarshals bytes into an Interval object
func (i *Interval) UnmarshalJSON(data []byte) (err error) {
	s := string(data)
	if s == "null" {
		return err
	}

	s, err = strconv.Unquote(s)
	if err != nil {
		return err
	}

	t, err := time.ParseDuration(s)
	if err != nil {
		return err
	}

	*i = Interval(t)

	return err
}

//MarshalJSON converts the Interval object to a JSON byte array
func (i Interval) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote(time.Duration(i).String())), nil
}
