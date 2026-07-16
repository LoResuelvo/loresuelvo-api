package provider

func WithProfilePhotoURLs(providers []Provider, profilePhotoURLs map[string]string) []Provider {
	for index := range providers {
		if providers[index].ProfilePhoto() == nil {
			continue
		}
		providers[index].SetProfilePhotoURL(profilePhotoURLs[providers[index].ProfilePhoto().FileID])
	}
	return providers
}
