package bsonwrap

import (
	"bytes"
	"fmt"

	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
)

// swagger:strfmt objectid
type ObjectID struct {
	primitive.ObjectID
}

// Default ObjectId
var NilObjectID ObjectID

// Check if ObjectId is NilObjectId or not
func (id ObjectID) Valid() bool {
	return !id.ObjectID.IsZero()
}

// Check if string have valid objectId (empty ObjectId is an invalid ObjectId)
func IsObjectIdHex(s string) bool {
	id, err := primitive.ObjectIDFromHex(s)
	if err != nil {
		return false
	}
	return !bytes.Equal(id[:], primitive.NilObjectID[:])
}

// Return valid ObjectId
// Panic if invalid objectId string is passed (empty string also invalid)
func ObjectIdHex(s string) ObjectID {
	if s == "" {
		return NilObjectID
	}
	id, err := primitive.ObjectIDFromHex(s)
	if err != nil {
		panic(fmt.Sprintf("invalid input to ObjectIdHex: %q", s))
	}

	return ObjectID{id}
}

// Creates a valid ObjectId
func NewObjectID() ObjectID {
	return ObjectID{primitive.NewObjectID()}
}

func (id *ObjectID) UnmarshalBSONValue(t bsontype.Type, raw []byte) error {
	if t == bsontype.ObjectID && len(raw) == 12 {
		var objID primitive.ObjectID
		copy(objID[:], raw)
		*id = ObjectID{objID}
		return nil
	} else if t == bsontype.String {
		if str, _, ok := bsoncore.ReadString(raw); ok && str == "" {
			*id = NilObjectID
			return nil
		}
	} else if t == bsontype.Null {
		*id = NilObjectID
		return nil
	}

	return fmt.Errorf("unable to unmarshal bson id &mdash; type: %v, length: %v", t, len(raw))
}

func (id ObjectID) MarshalBSONValue() (bsontype.Type, []byte, error) {
	if id == NilObjectID {
		return 0, nil, fmt.Errorf("object Id cannot be NilObjectId")
	}
	return bsontype.ObjectID, id.ObjectID[:], nil
}

// Covert ObjectId to byte array object of json (if its a valid ObjectId, else return error)
// Return empty string json for NilObjectID
func (id ObjectID) MarshalJSON() ([]byte, error) {
	if id == NilObjectID {
		return []byte(`""`), nil
	}
	return id.ObjectID.MarshalJSON()
}

// Convert json byte array object to ObjectId (if its a valid objectId json, else return error)
// Return NilObjectID for empty string json
func (id *ObjectID) UnmarshalJSON(b []byte) error {
	if string(b) == `""` {
		*id = NilObjectID
		return nil
	}
	return id.ObjectID.UnmarshalJSON(b)
}

func (id ObjectID) MarshalText() ([]byte, error) {
	if id.IsZero() {
		return []byte(""), nil
	}
	return []byte(id.Hex()), nil
}
