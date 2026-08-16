package file_handler

import (
	"errors"
	"net/http"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/gin-gonic/gin"
)

func handleFileError(c *gin.Context, err error) {
	if errors.Is(err, filedomain.ErrUnsupportedProfilePhoto) || errors.Is(err, filedomain.ErrProfilePhotoNotAvailable) {
		httphandler.RespondError(c, http.StatusBadRequest, filedomain.ErrProfilePhotoNotAvailable.Error())
		return
	}
	if errors.Is(err, filedomain.ErrMessageImageNotAvailable) {
		httphandler.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, filedomain.ErrWorkOrderCompletionImageNotAvailable) {
		httphandler.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, filedomain.ErrUnsupportedMessageAudio) || errors.Is(err, filedomain.ErrMessageAudioNotAvailable) {
		httphandler.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, filedomain.ErrUnsupportedMessageVideo) || errors.Is(err, filedomain.ErrMessageVideoNotAvailable) {
		httphandler.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, filedomain.ErrFileNotAvailable) {
		httphandler.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, filedomain.ErrProfilePhotoRequired) ||
		errors.Is(err, filedomain.ErrOriginalNameRequired) ||
		errors.Is(err, filedomain.ErrMimeTypeRequired) ||
		errors.Is(err, filedomain.ErrSizeRequired) ||
		errors.Is(err, filedomain.ErrPurposeRequired) ||
		errors.Is(err, filedomain.ErrUnsupportedPurpose) {
		httphandler.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	httphandler.RespondError(c, http.StatusInternalServerError, "internal server error")
}
