package pubsub

import (
	"strings"

	"github.com/anyproto/any-sync/commonspace/pubsub/pubsubproto"
)

const (
	maxTopicLen  = 256
	maxSegments  = 16
	wildcardOne  = "*"
	wildcardTail = ">"
	accNamespace = "acc"
)

// splitTopic splits a topic into segments using a stack-allocated array for the
// common case. Leading, trailing and doubled separators produce empty segments,
// which validation rejects.
func splitTopic(topic string) []string {
	var tsa [maxSegments]string
	n := 0
	rest := topic
	for n < maxSegments {
		idx := strings.IndexByte(rest, '/')
		if idx < 0 {
			tsa[n] = rest
			n++
			return tsa[:n:n]
		}
		tsa[n] = rest[:idx]
		n++
		rest = rest[idx+1:]
	}
	// over maxSegments: return an over-length slice so validation rejects it
	return append(tsa[:n:n], rest)
}

// validateSegments applies the shared structural rules for topics and patterns.
func validateSegments(topic string, segs []string) error {
	if len(topic) == 0 || len(topic) > maxTopicLen {
		return pubsubproto.ErrInvalidTopic
	}
	if len(segs) > maxSegments {
		return pubsubproto.ErrInvalidTopic
	}
	// canonical form: no leading/trailing separator, no empty segments
	for _, s := range segs {
		if s == "" {
			return pubsubproto.ErrInvalidTopic
		}
	}
	return nil
}

// ValidateTopic checks a fully-qualified publish topic: canonical form, no wildcards.
func ValidateTopic(topic string) error {
	segs := splitTopic(topic)
	if err := validateSegments(topic, segs); err != nil {
		return err
	}
	for _, s := range segs {
		if strings.ContainsAny(s, "*>") {
			return pubsubproto.ErrInvalidTopic
		}
	}
	return nil
}

// ValidatePattern checks a subscription pattern: canonical form, wildcards only as
// whole segments, '>' only in tail position.
func ValidatePattern(pattern string) error {
	segs := splitTopic(pattern)
	if err := validateSegments(pattern, segs); err != nil {
		return err
	}
	for i, s := range segs {
		switch s {
		case wildcardOne:
			continue
		case wildcardTail:
			if i != len(segs)-1 {
				return pubsubproto.ErrInvalidTopic
			}
		default:
			if strings.ContainsAny(s, "*>") {
				return pubsubproto.ErrInvalidTopic
			}
		}
	}
	return nil
}

// validateSpaceId rejects a spaceId that would break the "spaceId/pattern" tag
// encoding. spaceId must be non-empty and contain no '/'.
func validateSpaceId(spaceId string) error {
	if spaceId == "" || strings.IndexByte(spaceId, '/') >= 0 {
		return pubsubproto.ErrInvalidTopic
	}
	return nil
}

// TopicOwner returns the account id that exclusively may publish to the topic, or ""
// if the topic is not in the self-owned acc/ namespace. The owner is the last segment.
// Assumes a validated fully-qualified topic.
func TopicOwner(topic string) string {
	segs := splitTopic(topic)
	if len(segs) < 2 || segs[0] != accNamespace {
		return ""
	}
	return segs[len(segs)-1]
}
