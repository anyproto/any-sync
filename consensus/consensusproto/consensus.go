package consensusproto

func WrapHeadUpdate(update *LogHeadUpdate, rootRecord *RawRecordWithId) *LogSyncMessage {
	return &LogSyncMessage{
		Content: &LogSyncContentValue{
			Value: &LogSyncContentValue_HeadUpdate{HeadUpdate: update},
		},
		Id:      rootRecord.Id,
		Payload: rootRecord.Payload,
	}
}

func WrapFullRequest(request *LogFullSyncRequest, rootRecord *RawRecordWithId) *LogSyncMessage {
	return &LogSyncMessage{
		Content: &LogSyncContentValue{
			Value: &LogSyncContentValue_FullSyncRequest{FullSyncRequest: request},
		},
		Id:      rootRecord.Id,
		Payload: rootRecord.Payload,
	}
}

func WrapFullResponse(response *LogFullSyncResponse, rootRecord *RawRecordWithId) *LogSyncMessage {
	return &LogSyncMessage{
		Content: &LogSyncContentValue{
			Value: &LogSyncContentValue_FullSyncResponse{FullSyncResponse: response},
		},
		Id:      rootRecord.Id,
		Payload: rootRecord.Payload,
	}
}

func GetHead(msg *LogSyncMessage) (head string) {
	content := msg.GetContent()
	switch {
	case content.GetHeadUpdate() != nil:
		return content.GetHeadUpdate().Head
	case content.GetFullSyncRequest() != nil:
		return content.GetFullSyncRequest().Head
	case content.GetFullSyncResponse() != nil:
		return content.GetFullSyncResponse().Head
	default:
		return ""
	}
}
