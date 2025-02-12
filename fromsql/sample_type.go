package fromsql

import (
	"encoding/json"
	"fmt"
	"reflect"

	"net/mail"
	"net/url"

	"github.com/gofrs/uuid"
)

func (rr remoteRows) isUUID(columnName string) bool {
	hasAtLeastOne := false
	for _, r := range rr {
		value := r[columnName]
		if value != nil {
			if reflect.ValueOf(value).Kind() == reflect.String {
				if u := uuid.FromStringOrNil(fmt.Sprintf("%v", value)); u == uuid.Nil {
					return false
				} else {
					hasAtLeastOne = true
				}
			}
		}
	}
	return hasAtLeastOne
}

func (rr remoteRows) isEmail(columnName string) bool {
	hasAtLeastOne := false
	for _, r := range rr {
		value := r[columnName]
		if value != nil {
			if reflect.ValueOf(value).Kind() == reflect.String {
				if _, err := mail.ParseAddress(fmt.Sprintf("%v", value)); err != nil {
					return false
				} else {
					hasAtLeastOne = true
				}
			}
		}
	}
	return hasAtLeastOne
}

func (rr remoteRows) isURL(columnName string) bool {
	hasAtLeastOne := false
	for _, r := range rr {
		value := r[columnName]
		if value != nil {
			if reflect.ValueOf(value).Kind() == reflect.String {
				if _, err := url.Parse(fmt.Sprintf("%v", value)); err != nil {
					return false
				} else {
					hasAtLeastOne = true
				}
			}
		}
	}
	return hasAtLeastOne
}

func (rr remoteRows) isJSONArray(columnName string) bool {
	hasAtLeastOne := false
	for _, r := range rr {
		value := r[columnName]

		var v interface{}
		if err := json.Unmarshal([]byte(fmt.Sprintf("%v", value)), &v); err != nil {
			// handle error
		}

		switch v.(type) {
		case []interface{}:
			hasAtLeastOne = true
		case map[string]interface{}:
			return false
		default:
			return false
		}
	}
	return hasAtLeastOne
}
