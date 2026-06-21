package file

func validateUploadRequest(request PresignRequest, policy UploadPolicy) (*FileMetadata, error) {
	if request.AuthID == "" {
		return nil, ErrUploaderRequired
	}

	metadata, err := NewFileMetadata(request.OriginalName, request.MimeType, request.SizeBytes)
	if err != nil {
		return nil, err
	}
	if !policy.Allows(*metadata) {
		return nil, policy.InvalidMetadataError
	}
	return metadata, nil
}
