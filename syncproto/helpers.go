package syncproto

func WrapHeadUpdate(update *SyncHeadUpdate) *Sync {
	return &Sync{Message: &SyncContentValue{
		Value: &SyncContentValueValueOfHeadUpdate{HeadUpdate: update},
	}}
}

func WrapFullRequest(request *SyncFullRequest) *Sync {
	return &Sync{Message: &SyncContentValue{
		Value: &SyncContentValueValueOfFullSyncRequest{FullSyncRequest: request},
	}}
}

func WrapFullResponse(response *SyncFullResponse) *Sync {
	return &Sync{Message: &SyncContentValue{
		Value: &SyncContentValueValueOfFullSyncResponse{FullSyncResponse: response},
	}}
}
