package core_test

import (
	"context"
	"encoding/xml"
	"errors"
	"github.com/jzx17/gofetch/core"
	"github.com/jzx17/gofetch/utils/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"io"
	"net/http"
	"os"
	"strings"
)

var _ = Describe("Response", func() {
	It("should read JSON correctly", func() {
		jsonStr := `{"message": "hello"}`
		res := &http.Response{
			Status:     "200 OK",
			StatusCode: 200,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(jsonStr)),
		}
		response := &core.Response{Response: res}
		var result map[string]string
		err := response.JSON(&result)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(map[string]string{"message": "hello"}))
	})

	It("should read XML correctly", func() {
		xmlStr := `<root><message>hello</message></root>`
		res := &http.Response{
			Status:     "200 OK",
			StatusCode: 200,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(xmlStr)),
		}
		response := &core.Response{Response: res}

		type Root struct {
			XMLName xml.Name `xml:"root"`
			Message string   `xml:"message"`
		}

		var result Root
		err := response.XML(&result)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Message).To(Equal("hello"))
	})

	It("should read bytes correctly", func() {
		text := "some data"
		res := &http.Response{
			Status:     "200 OK",
			StatusCode: 200,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(text)),
		}
		response := &core.Response{Response: res}
		b, err := response.Bytes()
		Expect(err).NotTo(HaveOccurred())
		Expect(string(b)).To(Equal(text))
	})

	It("should read string correctly", func() {
		text := "some data"
		res := &http.Response{
			Status:     "200 OK",
			StatusCode: 200,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(text)),
		}
		response := &core.Response{Response: res}
		str, err := response.String()
		Expect(err).NotTo(HaveOccurred())
		Expect(str).To(Equal(text))
	})

	It("should save to file correctly", func() {
		text := "file data"
		res := &http.Response{
			Status:     "200 OK",
			StatusCode: 200,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(text)),
		}
		response := &core.Response{Response: res}
		tmpFile, err := os.CreateTemp("", "resp_test")
		Expect(err).NotTo(HaveOccurred())
		filePath := tmpFile.Name()
		tmpFile.Close()
		defer os.Remove(filePath)
		err = response.SaveToFile(filePath)
		Expect(err).NotTo(HaveOccurred())
		data, err := os.ReadFile(filePath)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(Equal(text))
	})

	It("should process the response body correctly", func() {
		text := "some data"
		res := &http.Response{
			Status:     "200 OK",
			StatusCode: 200,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(text)),
		}
		response := &core.Response{Response: res}

		var result string
		err := response.Process(func(reader io.Reader) error {
			data, err := io.ReadAll(reader)
			if err != nil {
				return err
			}
			result = string(data)
			return nil
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(text))
	})

	It("should stream chunks correctly with default buffer size", func() {
		text := "line1\nline2\nline3\n"
		res := &http.Response{
			Status:     "200 OK",
			StatusCode: 200,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(text)),
		}
		response := &core.Response{Response: res}
		var chunks []string
		err := response.StreamChunks(func(chunk []byte) {
			chunks = append(chunks, string(chunk))
		})
		Expect(err).NotTo(HaveOccurred())
		// With default buffer size (4096), we should get all the text in one chunk
		Expect(len(chunks)).To(Equal(1))
		Expect(chunks[0]).To(Equal(text))
	})

	It("should stream chunks correctly with optional buffer size", func() {
		// Use a small buffer size to force multiple chunks.
		text := "line1\nline2\nline3\n"
		res := &http.Response{
			Status:     "200 OK",
			StatusCode: 200,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(text)),
		}
		response := &core.Response{Response: res}
		var chunks []string
		err := response.StreamChunks(func(chunk []byte) {
			chunks = append(chunks, string(chunk))
		}, core.WithBufferSize(6))
		Expect(err).NotTo(HaveOccurred())
		Expect(len(chunks)).To(BeNumerically(">", 1), "Should have multiple chunks with small buffer size")
	})

	It("should ignore invalid buffer size values", func() {
		text := "some data"

		// Test with zero buffer size
		res1 := &http.Response{
			Status:     "200 OK",
			StatusCode: 200,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(text)),
		}
		response1 := &core.Response{Response: res1}

		var chunks1 []string
		err := response1.StreamChunks(func(chunk []byte) {
			chunks1 = append(chunks1, string(chunk))
		}, core.WithBufferSize(0))

		Expect(err).NotTo(HaveOccurred())
		Expect(chunks1).NotTo(BeEmpty(), "Should have collected at least one chunk")

		// Test with negative buffer size
		res2 := &http.Response{
			Status:     "200 OK",
			StatusCode: 200,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(text)),
		}
		response2 := &core.Response{Response: res2}

		var chunks2 []string
		err = response2.StreamChunks(func(chunk []byte) {
			chunks2 = append(chunks2, string(chunk))
		}, core.WithBufferSize(-1))

		Expect(err).NotTo(HaveOccurred())
		Expect(chunks2).NotTo(BeEmpty(), "Should have collected at least one chunk")
	})

	It("should immediately return context error if context is cancelled before streaming", func() {
		// Create a response with any non-blocking reader.
		res := &http.Response{
			Status:     "200 OK",
			StatusCode: 200,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("some data")),
		}
		response := &core.Response{Response: res}

		// Create and cancel the context immediately.
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		// Since context is already cancelled, StreamChunksWithContext should return immediately.
		err := response.StreamChunksWithContext(ctx, func(chunk []byte) {
			// This callback should not be invoked.
		})
		Expect(err).To(MatchError(context.Canceled))
	})

	It("should update BytesRead correctly during streaming", func() {
		text := "this is exactly 33 bytes of data."
		res := &http.Response{
			Status:     "200 OK",
			StatusCode: 200,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(text)),
		}
		response := &core.Response{Response: res}

		err := response.StreamChunks(func(chunk []byte) {
			// Do nothing with the chunks
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(response.BytesRead).To(Equal(int64(33)), "BytesRead should equal the length of the text")
	})

	It("should error from Bytes if underlying reader returns an error", func() {
		res := &http.Response{
			Status: "200 OK",
			Body:   io.NopCloser(test.NewErrorReader(errors.New("read error"))),
		}
		response := &core.Response{Response: res}
		_, err := response.Bytes()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("read error"))
	})

	It("should return error from StreamChunks if underlying reader error", func() {
		reader := test.NewStreamErrorReader([]byte("data"), errors.New("stream read error"))
		res := &http.Response{
			Status: "200 OK",
			Body:   reader,
		}
		response := &core.Response{Response: res}
		err := response.StreamChunks(func(chunk []byte) {})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("stream read error"))
	})

	It("should return error from StreamChunksWithContext if underlying reader error", func() {
		reader := test.NewStreamErrorReader([]byte("data"), errors.New("stream read error"))
		res := &http.Response{
			Status: "200 OK",
			Body:   reader,
		}
		response := &core.Response{Response: res}
		err := response.StreamChunksWithContext(context.Background(), func(chunk []byte) {})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("stream read error"))
	})

	It("should error if JSON decoding fails", func() {
		invalidJSON := `{"message": "hello"` // missing closing brace
		res := &http.Response{
			Status: "200 OK",
			Body:   io.NopCloser(strings.NewReader(invalidJSON)),
		}
		response := &core.Response{Response: res}
		var data map[string]string
		err := response.JSON(&data)
		Expect(err).To(HaveOccurred())
	})

	It("should error if XML decoding fails", func() {
		invalidXML := `<root><message>hello</root>` // missing closing message tag
		res := &http.Response{
			Status: "200 OK",
			Body:   io.NopCloser(strings.NewReader(invalidXML)),
		}
		response := &core.Response{Response: res}
		var data struct {
			XMLName xml.Name `xml:"root"`
			Message string   `xml:"message"`
		}
		err := response.XML(&data)
		Expect(err).To(HaveOccurred())
	})

	It("should error from SaveToFile if file creation fails", func() {
		res := &http.Response{
			Status: "200 OK",
			Body:   io.NopCloser(strings.NewReader("data")),
		}
		response := &core.Response{Response: res}
		// Use an invalid file path.
		err := response.SaveToFile("/invalid/path/to/file")
		Expect(err).To(HaveOccurred())
	})

	It("should return error from Bytes if closing the body fails", func() {
		res := &http.Response{
			Status: "200 OK",
			Body:   test.NewErrorCloser([]byte("data"), errors.New("failed to close response body")),
		}
		response := &core.Response{Response: res}
		_, err := response.Bytes()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed to close response body"))
	})

	It("should return error from JSON if closing the body fails", func() {
		res := &http.Response{
			Status: "200 OK",
			Body:   test.NewErrorCloser([]byte(`{"key":"value"}`), errors.New("failed to close response body")),
		}
		response := &core.Response{Response: res}
		var data map[string]string
		err := response.JSON(&data)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed to close response body"))
	})

	It("should return error from XML if closing the body fails", func() {
		res := &http.Response{
			Status: "200 OK",
			Body:   test.NewErrorCloser([]byte(`<root><message>hello</message></root>`), errors.New("failed to close response body")),
		}
		response := &core.Response{Response: res}
		var data struct {
			XMLName xml.Name `xml:"root"`
			Message string   `xml:"message"`
		}
		err := response.XML(&data)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed to close response body"))
	})

	It("should error from Process if the response is nil", func() {
		response := &core.Response{Response: nil}
		err := response.Process(func(reader io.Reader) error {
			return nil
		})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("nil response"))
	})

	It("should return error from Process if the processing function returns an error", func() {
		res := &http.Response{
			Status: "200 OK",
			Body:   io.NopCloser(strings.NewReader("data")),
		}
		response := &core.Response{Response: res}
		expectedErr := errors.New("processing error")
		err := response.Process(func(reader io.Reader) error {
			return expectedErr
		})
		Expect(err).To(Equal(expectedErr))
	})

	Context("Status code helper methods", func() {
		It("should correctly identify successful responses", func() {
			codes := []int{200, 201, 204, 299}
			for _, code := range codes {
				res := &http.Response{StatusCode: code}
				response := &core.Response{Response: res}
				Expect(response.IsSuccess()).To(BeTrue())
				Expect(response.IsError()).To(BeFalse())
			}
		})

		It("should correctly identify redirect responses", func() {
			codes := []int{301, 302, 303, 307, 308}
			for _, code := range codes {
				res := &http.Response{StatusCode: code}
				response := &core.Response{Response: res}
				Expect(response.IsRedirect()).To(BeTrue())
				Expect(response.IsError()).To(BeFalse())
			}
		})

		It("should correctly identify client error responses", func() {
			codes := []int{400, 401, 403, 404, 429}
			for _, code := range codes {
				res := &http.Response{StatusCode: code}
				response := &core.Response{Response: res}
				Expect(response.IsClientError()).To(BeTrue())
				Expect(response.IsError()).To(BeTrue())
				Expect(response.IsServerError()).To(BeFalse())
			}
		})

		It("should correctly identify server error responses", func() {
			codes := []int{500, 502, 503, 504}
			for _, code := range codes {
				res := &http.Response{StatusCode: code}
				response := &core.Response{Response: res}
				Expect(response.IsServerError()).To(BeTrue())
				Expect(response.IsError()).To(BeTrue())
				Expect(response.IsClientError()).To(BeFalse())
			}
		})
	})

	Context("MustSuccess", func() {
		It("should return the response if the status code is 2xx", func() {
			res := &http.Response{
				Status:     "200 OK",
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader("success")),
			}
			response := &core.Response{Response: res}
			result, err := response.MustSuccess()
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(response))
		})

		It("should return an error if the status code is not 2xx", func() {
			res := &http.Response{
				Status:     "404 Not Found",
				StatusCode: 404,
				Body:       io.NopCloser(strings.NewReader("not found")),
			}
			response := &core.Response{Response: res}
			_, err := response.MustSuccess()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("request failed with status 404 Not Found"))
			Expect(err.Error()).To(ContainSubstring("not found"))
		})
	})

	Context("CloseBody", func() {
		It("should return nil if Response is nil", func() {
			response := &core.Response{Response: nil}
			err := response.CloseBody()
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return nil if Body is nil", func() {
			res := &http.Response{
				Status:     "200 OK",
				StatusCode: 200,
				Body:       nil,
			}
			response := &core.Response{Response: res}
			err := response.CloseBody()
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return error if Body close fails", func() {
			errorCloser := test.NewErrorCloser([]byte("data"), errors.New("close error"))
			res := &http.Response{
				Status:     "200 OK",
				StatusCode: 200,
				Body:       errorCloser,
			}
			response := &core.Response{Response: res}
			err := response.CloseBody()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("close error"))
		})
	})

	Context("AsyncResponse", func() {
		It("should store a response and error", func() {
			res := &http.Response{
				Status:     "200 OK",
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader("data")),
			}
			response := &core.Response{Response: res}
			asyncResp := &core.AsyncResponse{
				Response: response,
				Error:    nil,
			}

			Expect(asyncResp.Response).To(Equal(response))
			Expect(asyncResp.Error).To(BeNil())

			// Test with error
			expectedErr := errors.New("async error")
			asyncRespWithErr := &core.AsyncResponse{
				Response: nil,
				Error:    expectedErr,
			}

			Expect(asyncRespWithErr.Response).To(BeNil())
			Expect(asyncRespWithErr.Error).To(Equal(expectedErr))
		})
	})
})
