package syncproto

func WrapHeadUpdate(update *Sync_HeadUpdate) *Sync {
	return &Sync{Message: &Sync_ContentValue{
		Value: &Sync_Content_Value_HeadUpdate{HeadUpdate: update},
	}}
}

func WrapFullRequest(request *Sync_Full_Request) *Sync {
	return &Sync{Message: &Sync_ContentValue{
		Value: &Sync_Content_Value_FullSyncRequest{FullSyncRequest: request},
	}}
}

func WrapFullResponse(response *Sync_Full_Response) *Sync {
	return &Sync{Message: &Sync_ContentValue{
		Value: &Sync_Content_Value_FullSyncResponse{FullSyncResponse: response},
	}}
}
