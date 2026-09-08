package chatbot

import (
	"github.com/stretchr/testify/mock"
	"net/http"
)

type generationTransportMock struct{ mock.Mock }

func (m *generationTransportMock) RoundTrip(request *http.Request) (*http.Response, error) {
	args := m.Called(request)
	response, _ := args.Get(0).(*http.Response)
	return response, args.Error(1)
}
