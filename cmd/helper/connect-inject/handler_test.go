package connectinject

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mattbaird/jsonpatch"
	"github.com/stretchr/testify/require"
	"k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestHandlerHandle(t *testing.T) {
	cases := []struct {
		Name    string
		Handler Handler
		Req     v1beta1.AdmissionRequest
		Err     string // expected error string, not exact
		Patches []jsonpatch.JsonPatchOperation
	}{
		{
			"kube-system namespace",
			Handler{},
			v1beta1.AdmissionRequest{
				Object: encodeRaw(t, &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: metav1.NamespaceSystem,
					},
				}),
			},
			"",
			nil,
		},

		{
			"empty pod",
			Handler{},
			v1beta1.AdmissionRequest{
				Object: encodeRaw(t, &corev1.Pod{
					Spec: corev1.PodSpec{},
				}),
			},
			"",
			[]jsonpatch.JsonPatchOperation{
				{
					Operation: "add",
					Path:      "/spec/containers",
				},
				{
					Operation: "add",
					Path:      "/metadata/annotations",
				},
			},
		},

		{
			"empty pod with injection disabled",
			Handler{},
			v1beta1.AdmissionRequest{
				Object: encodeRaw(t, &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							annotationInject: "false",
						},
					},
				}),
			},
			"",
			nil,
		},

		{
			"empty pod with injection truthy",
			Handler{},
			v1beta1.AdmissionRequest{
				Object: encodeRaw(t, &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							annotationInject: "t",
						},
					},
				}),
			},
			"",
			[]jsonpatch.JsonPatchOperation{
				{
					Operation: "add",
					Path:      "/spec/containers",
				},
				{
					Operation: "add",
					Path:      "/metadata/annotations",
				},
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.Name, func(t *testing.T) {
			require := require.New(t)
			resp := tt.Handler.Mutate(&tt.Req)
			if (tt.Err == "") != resp.Allowed {
				t.Fatalf("allowed: %v, expected err: %v", resp.Allowed, tt.Err)
			}
			if tt.Err != "" {
				require.Contains(resp.Result.Message, tt.Err)
				return
			}

			var actual []jsonpatch.JsonPatchOperation
			if len(resp.Patch) > 0 {
				require.NoError(json.Unmarshal(resp.Patch, &actual))
				for i, _ := range actual {
					actual[i].Value = nil
				}
			}
			require.Equal(actual, tt.Patches)
		})
	}
}

// Test that an incorrect content type results in an error.
func TestHandlerHandle_badContentType(t *testing.T) {
	req, err := http.NewRequest("POST", "/", nil)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "text/plain")

	var h Handler
	rec := httptest.NewRecorder()
	h.Handle(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "content-type")
}

// Test that no body results in an error
func TestHandlerHandle_noBody(t *testing.T) {
	req, err := http.NewRequest("POST", "/", nil)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	var h Handler
	rec := httptest.NewRecorder()
	h.Handle(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "body")
}

// encodeRaw is a helper to encode some data into a RawExtension.
func encodeRaw(t *testing.T, input interface{}) runtime.RawExtension {
	data, err := json.Marshal(input)
	require.NoError(t, err)
	return runtime.RawExtension{Raw: data}
}