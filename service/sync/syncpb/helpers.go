package syncpb

func WrapHeadUpdate(update *SyncHeadUpdate) *SyncContent {
	return &SyncContent{Message: &SyncContentValue{
		Value: &SyncContentValueValueOfHeadUpdate{HeadUpdate: update},
	}}
}

func WrapFullRequest(request *SyncFullRequest) *SyncContent {
	return &SyncContent{Message: &SyncContentValue{
		Value: &SyncContentValueValueOfFullSyncRequest{FullSyncRequest: request},
	}}
}

func WrapFullResponse(response *SyncFullResponse) *SyncContent {
	return &SyncContent{Message: &SyncContentValue{
		Value: &SyncContentValueValueOfFullSyncResponse{FullSyncResponse: response},
	}}
}
