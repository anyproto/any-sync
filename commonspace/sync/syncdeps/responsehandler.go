package syncdeps

import "context"

type ResponseHandler interface {
	NewResponse() Response
	HandleResponse(ctx context.Context, peerId, objectId string, resp Response) error
}
