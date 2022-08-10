package tree

import (
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"github.com/gogo/protobuf/proto"
	"strings"
	"unicode"
)

type DescriptionParser interface {
	ParseChange(*Change) ([]string, error)
}

var ACLDescriptionParser = aclDescriptionParser{}

type aclDescriptionParser struct{}

func (a aclDescriptionParser) ParseChange(changeWrapper *Change) (res []string, err error) {
	change := changeWrapper.Content
	aclData := &aclpb.ACLChangeACLData{}

	if changeWrapper.ParsedModel != nil {
		aclData = changeWrapper.ParsedModel.(*aclpb.ACLChangeACLData)
	} else {
		err = proto.Unmarshal(change.ChangesData, aclData)
		if err != nil {
			return
		}
	}

	var chSymbs []string
	for _, chc := range aclData.AclContent {
		tp := fmt.Sprintf("%T", chc.Value)
		tp = strings.Replace(tp, "ACLChangeACLContentValueValueOf", "", 1)
		res := ""
		for _, ts := range tp {
			if unicode.IsUpper(ts) {
				res += string(ts)
			}
		}
		chSymbs = append(chSymbs, res)
	}
	return chSymbs, nil
}
