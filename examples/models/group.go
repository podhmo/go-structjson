package models

import "gopkg.in/mgo.v2/bson"

type Group struct {
	ID   bson.ObjectId `json:"id" bson:"_id"`
	Name string        `json:"name"`
}

func init() {
}
