package email

import (
	"github.com/go-openapi/strfmt"
	"gopkg.in/mgo.v2/bson"
)

// Email メールアドレス
type Email strfmt.Email

func f(s string) bson.ObjectId {
	return bson.ObjectIdHex(s)
}
