package models

import (
	"gopkg.in/mgo.v2/bson"
)

// PersonGender : gender
type PersonGender string

// PersonGender : constants
const (
	PersonGenderFemale  = PersonGender("female")
	PersonGendermale    = PersonGender("male")
	PersonGenderUnknown = PersonGender("unknown")
)

// Person : person model
type Person struct {
	ID     bson.ObjectId `json:"id" bson:"_id"`
	Name   string        `json:"name" bson:"name"`
	Age    int           `json:"age" bson:"age"`
	Gender PersonGender  `json:"gender" bson:"gender"`
}
