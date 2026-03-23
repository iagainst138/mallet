package mallet

import "encoding/json"

func ParseJson(b []byte) error {
	var response any

	if err := json.Unmarshal(b, &response); err != nil {
		return err
	}

	return nil
}
