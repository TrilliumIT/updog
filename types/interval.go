package types

import (
	"strconv"
	"time"
)

type Interval time.Duration

func (i *Interval) UnmarshalJSON(data []byte) (err error) {
	s := string(data)
	if s == "null" {
		return
	}

	s, err = strconv.Unquote(s)
	if err != nil {
		return
	}

	t, err := time.ParseDuration(s)
	if err != nil {
		return
	}

	*i = Interval(t)

	return
}

func (i Interval) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote(time.Duration(i).String())), nil
}
