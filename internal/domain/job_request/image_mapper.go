package jobrequest

import (
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/job_request/read_model"
)

func fileImagesFromReadModelImages(images []readmodel.JobRequestImage) []filedomain.Image {
	result := make([]filedomain.Image, 0, len(images))
	for _, image := range images {
		result = append(result, filedomain.Image{FileID: image.FileID, OriginalName: image.OriginalName, URL: image.URL})
	}
	return result
}

func readModelImagesFromFileImages(images []filedomain.Image) []readmodel.JobRequestImage {
	result := make([]readmodel.JobRequestImage, 0, len(images))
	for _, image := range images {
		result = append(result, readmodel.JobRequestImage{FileID: image.FileID, OriginalName: image.OriginalName, URL: image.URL})
	}
	return result
}
