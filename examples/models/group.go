package models

import b "gopkg.in/mgo.v2/bson"

type Group struct {
	ID   b.ObjectId `json:"id" bson:"_id"`
	Name string     `json:"name"`
}

func init() {
}
