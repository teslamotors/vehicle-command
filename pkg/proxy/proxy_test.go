package proxy_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/jarcoal/httpmock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	"github.com/teslamotors/vehicle-command/internal/authentication"
	"github.com/teslamotors/vehicle-command/mocks"
	"github.com/teslamotors/vehicle-command/pkg/protocol"
	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/carserver"
	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/vcsec"
	"github.com/teslamotors/vehicle-command/pkg/proxy"
)

const (
	vin = "TESLA000000000001"
)

var (
	validJwt           = "x." + b64Encode(fmt.Sprintf(`{"aud": ["%s"]}`, "example.com")) + ".y"
	authorizationToken = "Bearer " + validJwt
)

func b64Encode(payload string) string {
	return base64.RawStdEncoding.EncodeToString([]byte(payload))
}

var _ = Describe("Proxy", func() {
	var (
		ctrl        *gomock.Controller
		p           *proxy.Proxy
		mockAccount *mocks.ProxyAccount
		validToken  bool
		signerKey   authentication.ECDHPrivateKey
	)

	sendRequest := func(method, path string, token string, body []byte) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, bytes.NewReader(body))
		req.Header.Set("Authorization", token)
		rr := httptest.NewRecorder()
		p.ServeHTTP(rr, req)
		return rr
	}

	BeforeEach(func() {
		var err error
		validToken = true
		ctrl = gomock.NewController(GinkgoT())
		mockAccount = mocks.NewProxyAccount(ctrl)
		signerKey, err = authentication.NewECDHPrivateKey(rand.Reader)
		Expect(err).NotTo(HaveOccurred())
		p = proxy.New(context.Background(), signerKey, 0, func(oauthToken, userAgent string) (proxy.Account, error) {
			if validToken {
				return mockAccount, nil
			}
			return nil, fmt.Errorf("invalid token")
		})
		Expect(err).ToNot(HaveOccurred())
		DeferCleanup(func() {
			ctrl.Finish()
		})
	})

	Context("vehicle commands", func() {
		Context("invalid VIN", func() {
			It("returns not found", func() {
				rr := sendRequest(http.MethodPost, "/api/1/vehicles/ABC/command/honk_horn", authorizationToken, nil)
				Expect(rr.Code).To(Equal(http.StatusNotFound))
			})
		})

		Context("invalid auth token", func() {
			It("returns unauthorized", func() {
				validToken = false
				rr := sendRequest(http.MethodPost, "/api/1/vehicles/ABC/command/honk_horn", "Bearer invalid", nil)
				Expect(rr.Code).To(Equal(http.StatusUnauthorized))
			})
		})

		Context("signed command", func() {
			It("returns successful response", func() {
				vehicle := mocks.NewProxyVehicle(ctrl)
				vehicle.EXPECT().Connect(gomock.Any()).Return(nil)
				vehicle.EXPECT().StartSession(gomock.Any(), gomock.Any()).Return(nil)
				vehicle.EXPECT().ExecuteAction(gomock.Any(), gomock.AssignableToTypeOf(&carserver.Action_VehicleAction{})).Return(nil)
				vehicle.EXPECT().UpdateCachedSessions(gomock.Any()).Return(nil)
				vehicle.EXPECT().Disconnect()
				mockAccount.EXPECT().GetVehicle(gomock.Any(), vin, gomock.Any(), gomock.Any()).Return(vehicle, nil)

				rr := sendRequest(http.MethodPost, fmt.Sprintf("/api/1/vehicles/%s/command/honk_horn", vin), authorizationToken, nil)
				Expect(rr.Code).To(Equal(http.StatusOK))
				Expect(rr.Body.String()).To(MatchJSON(`{"response":{"result":true,"reason":""}}`))
			})
		})

		Context("unsigned command", func() {
			It("returns successful response", func() {
				vehicle := mocks.NewProxyVehicle(ctrl)
				vehicle.EXPECT().Connect(gomock.Any()).Return(nil)
				vehicle.EXPECT().StartSession(gomock.Any(), gomock.Any()).Return(nil)
				vehicle.EXPECT().ExecuteAction(gomock.Any(), gomock.AssignableToTypeOf(&vcsec.UnsignedMessage{})).Return(nil)
				vehicle.EXPECT().UpdateCachedSessions(gomock.Any()).Return(nil)
				vehicle.EXPECT().Disconnect()
				mockAccount.EXPECT().GetVehicle(gomock.Any(), vin, gomock.Any(), gomock.Any()).Return(vehicle, nil)

				rr := sendRequest(http.MethodPost, fmt.Sprintf("/api/1/vehicles/%s/command/door_lock", vin), authorizationToken, nil)
				Expect(rr.Code).To(Equal(http.StatusOK))
				Expect(rr.Body.String()).To(MatchJSON(`{"response":{"result":true,"reason":""}}`))
			})
		})

		Context("signed command not supported", func() {
			It("forwards unsigned command", func() {
				httpmock.Activate()
				defer httpmock.DeactivateAndReset()

				vehicle := mocks.NewProxyVehicle(ctrl)
				vehicle.EXPECT().Connect(gomock.Any()).Return(nil)
				vehicle.EXPECT().StartSession(gomock.Any(), gomock.Any()).Return(protocol.ErrProtocolNotSupported)
				mockAccount.EXPECT().GetHost().Return("example.com")
				httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("https://%s/api/1/vehicles/%s/command/honk_horn", "example.com", vin), func(r *http.Request) (*http.Response, error) {
					Expect(r.Header.Get("Authorization")).To(Equal("Bearer " + validJwt))
					return httpmock.NewJsonResponse(http.StatusOK, map[string]interface{}{
						"response": map[string]interface{}{
							"result": true,
							"reason": "",
						},
					})
				})
				vehicle.EXPECT().Disconnect()
				mockAccount.EXPECT().GetVehicle(gomock.Any(), vin, gomock.Any(), gomock.Any()).Return(vehicle, nil)

				rr := sendRequest(http.MethodPost, fmt.Sprintf("/api/1/vehicles/%s/command/honk_horn", vin), authorizationToken, nil)
				Expect(rr.Code).To(Equal(http.StatusOK))
				Expect(rr.Body.String()).To(MatchJSON(`{"response":{"result":true,"reason":""}}`))
			})

			It("does not try establishing session on subsequent requests for vehicles that do not support signed commands", func() {
				httpmock.Activate()
				defer httpmock.DeactivateAndReset()

				vehicle := mocks.NewProxyVehicle(ctrl)
				vehicle.EXPECT().Connect(gomock.Any()).Return(nil).Times(1)
				vehicle.EXPECT().StartSession(gomock.Any(), gomock.Any()).Return(protocol.ErrProtocolNotSupported).Times(1)
				vehicle.EXPECT().Disconnect().Times(1)
				httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("https://%s/api/1/vehicles/%s/command/honk_horn", "example.com", vin), func(r *http.Request) (*http.Response, error) {
					Expect(r.Header.Get("Authorization")).To(Equal("Bearer " + validJwt))
					return httpmock.NewJsonResponse(http.StatusOK, map[string]interface{}{
						"response": map[string]interface{}{
							"result": true,
							"reason": "",
						},
					})
				})

				mockAccount.EXPECT().GetHost().Return("example.com").AnyTimes()
				mockAccount.EXPECT().GetVehicle(gomock.Any(), vin, gomock.Any(), gomock.Any()).Return(vehicle, nil)

				for i := 0; i < 3; i++ {
					rr := sendRequest(http.MethodPost, fmt.Sprintf("/api/1/vehicles/%s/command/honk_horn", vin), authorizationToken, nil)
					Expect(rr.Code).To(Equal(http.StatusOK))
					Expect(rr.Body.String()).To(MatchJSON(`{"response":{"result":true,"reason":""}}`))
				}
			})

			It("returns errors", func() {
				httpmock.Activate()
				defer httpmock.DeactivateAndReset()

				vehicle := mocks.NewProxyVehicle(ctrl)
				vehicle.EXPECT().Connect(gomock.Any()).Return(nil)
				vehicle.EXPECT().StartSession(gomock.Any(), gomock.Any()).Return(protocol.ErrProtocolNotSupported)
				mockAccount.EXPECT().GetHost().Return("example.com")
				httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("https://%s/api/1/vehicles/%s/command/honk_horn", "example.com", vin), func(r *http.Request) (*http.Response, error) {
					Expect(r.Header.Get("Authorization")).To(Equal("Bearer " + validJwt))
					return httpmock.NewJsonResponse(http.StatusRequestTimeout, map[string]interface{}{
						"error": "vehicle offline",
					})
				})
				vehicle.EXPECT().Disconnect()
				mockAccount.EXPECT().GetVehicle(gomock.Any(), vin, gomock.Any(), gomock.Any()).Return(vehicle, nil)

				rr := sendRequest(http.MethodPost, fmt.Sprintf("/api/1/vehicles/%s/command/honk_horn", vin), authorizationToken, nil)
				Expect(rr.Code).To(Equal(http.StatusRequestTimeout))
				Expect(rr.Body.String()).To(MatchJSON(`{"error":"vehicle offline"}`))
			})

			It("fails for unknown command", func() {
				rr := sendRequest(http.MethodPost, fmt.Sprintf("/api/1/vehicles/%s/command/unknown", vin), authorizationToken, nil)
				Expect(rr.Code).To(Equal(http.StatusNotFound))
			})
		})
	})

	Describe("fleet telemetry config", func() {
		Context("invalid json body", func() {
			It("returns 400 bad request", func() {
				req := httptest.NewRequest(http.MethodPost, "/api/1/vehicles/fleet_telemetry_config", bytes.NewReader([]byte("invalid")))
				req.Header.Set("Authorization", authorizationToken)

				rr := httptest.NewRecorder()
				p.ServeHTTP(rr, req)

				Expect(rr.Code).To(Equal(http.StatusBadRequest))
			})
		})

		It("signs and forwards jws", func() {
			httpmock.Activate()
			defer httpmock.DeactivateAndReset()

			httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("https://%s/api/1/vehicles/fleet_telemetry_config_jws", "example.com"), func(r *http.Request) (*http.Response, error) {
				Expect(r.Header.Get("Authorization")).To(Equal("Bearer " + validJwt))

				type Body struct {
					Token string   `json:"token"`
					Vins  []string `json:"vins"`
				}
				var body Body
				bodyBytes, err := io.ReadAll(r.Body)
				Expect(err).ToNot(HaveOccurred())
				defer r.Body.Close()

				err = json.Unmarshal(bodyBytes, &body)
				Expect(err).ToNot(HaveOccurred())

				Expect(body.Vins).To(Equal([]string{vin}))
				tokenParts := strings.Split(body.Token, ".")
				Expect(len(tokenParts)).To(Equal(3))

				var claims map[string]interface{}
				claimsStr, err := base64.RawURLEncoding.DecodeString(tokenParts[1])
				Expect(err).ToNot(HaveOccurred())
				err = json.Unmarshal(claimsStr, &claims)

				Expect(claims["aud"]).To(Equal("com.tesla.fleet.TelemetryClient"))
				Expect(claims["iss"]).To(Equal(base64.StdEncoding.EncodeToString(signerKey.PublicBytes())))
				Expect(claims["fields"]).To(Equal(map[string]interface{}{
					"Soc": map[string]interface{}{
						"interval_seconds": float64(1),
					},
				}))

				return httpmock.NewJsonResponse(http.StatusOK, map[string]interface{}{
					"response": map[string]interface{}{
						"updated_vehicles": 1,
					},
				})
			})

			mockAccount.EXPECT().GetHost().Return("example.com")

			body := map[string]interface{}{
				"vins": []string{vin},
				"config": map[string]interface{}{
					"fields": map[string]interface{}{
						"Soc": map[string]interface{}{
							"interval_seconds": 1,
						},
					},
					"aud": "should be overwritten",
					"iss": "should be overwritten",
				},
			}
			bodyBytes, err := json.Marshal(body)
			Expect(err).ToNot(HaveOccurred())
			rr := sendRequest(http.MethodPost, "/api/1/vehicles/fleet_telemetry_config", authorizationToken, bodyBytes)
			Expect(rr.Code).To(Equal(http.StatusOK))
			Expect(rr.Body.String()).To(MatchJSON(`{"response":{"updated_vehicles":1}}`))
		})
	})

	Describe("forward request", func() {
		Describe("X-Forwarded-For header", func() {
			It("adds header", func() {
				httpmock.Activate()
				defer httpmock.DeactivateAndReset()

				httpmock.RegisterResponder(http.MethodGet, "https://example.com/api/1/unknown", func(r *http.Request) (*http.Response, error) {
					Expect(r.Header.Get("X-Forwarded-For")).To(Equal("1.2.3.4"))
					return httpmock.NewJsonResponse(http.StatusOK, map[string]interface{}{})
				})

				req := httptest.NewRequest(http.MethodGet, "/api/1/unknown", nil)
				req.Header.Set("Authorization", authorizationToken)
				req.RemoteAddr = "1.2.3.4:5678"
				mockAccount.EXPECT().GetHost().Return("example.com")

				rr := httptest.NewRecorder()
				p.ServeHTTP(rr, req)

				Expect(rr.Code).To(Equal(http.StatusOK))
			})

			It("adds to existing X-Forwarded-For header", func() {
				httpmock.Activate()
				defer httpmock.DeactivateAndReset()

				httpmock.RegisterResponder(http.MethodGet, "https://example.com/api/1/unknown", func(r *http.Request) (*http.Response, error) {
					Expect(r.Header.Get("X-Forwarded-For")).To(Equal("5.6.7.8, 9.10.11.12, 1.2.3.4"))
					return httpmock.NewJsonResponse(http.StatusOK, map[string]interface{}{})
				})

				req := httptest.NewRequest(http.MethodGet, "/api/1/unknown", nil)
				req.Header.Set("Authorization", authorizationToken)
				req.RemoteAddr = "1.2.3.4:5678"
				req.Header.Set("X-Forwarded-For", "5.6.7.8, 9.10.11.12")
				mockAccount.EXPECT().GetHost().Return("example.com")

				rr := httptest.NewRecorder()
				p.ServeHTTP(rr, req)

				Expect(rr.Code).To(Equal(http.StatusOK))
			})
		})

		Describe("per-hop headers", func() {
			It("removes before forwarding", func() {
				httpmock.Activate()
				defer httpmock.DeactivateAndReset()

				httpmock.RegisterResponder(http.MethodGet, "https://example.com/api/1/unknown", func(r *http.Request) (*http.Response, error) {
					Expect(r.Header.Get("Proxy-Connection")).To(Equal(""))
					Expect(r.Header.Get("Keep-Alive")).To(Equal(""))
					Expect(r.Header.Get("Transfer-Encoding")).To(Equal(""))
					Expect(r.Header.Get("Te")).To(Equal(""))
					Expect(r.Header.Get("Upgrade")).To(Equal(""))
					Expect(r.Header.Get("X-TXID")).To(Equal("abc123"))
					return httpmock.NewJsonResponse(http.StatusOK, map[string]interface{}{})
				})

				req := httptest.NewRequest(http.MethodGet, "/api/1/unknown", nil)
				req.Header.Set("Authorization", authorizationToken)
				req.Header.Set("Proxy-Connection", "keep-alive")
				req.Header.Set("Keep-Alive", "timeout=5")
				req.Header.Set("Transfer-Encoding", "chunked")
				req.Header.Set("Te", "trailers")
				req.Header.Set("Upgrade", "websocket")
				req.Header.Set("X-TXID", "abc123")
				mockAccount.EXPECT().GetHost().Return("example.com")

				rr := httptest.NewRecorder()
				p.ServeHTTP(rr, req)

				Expect(rr.Code).To(Equal(http.StatusOK))
			})

			It("removes from response", func() {
				httpmock.Activate()
				defer httpmock.DeactivateAndReset()

				httpmock.RegisterResponder(http.MethodGet, "https://example.com/api/1/unknown", func(r *http.Request) (*http.Response, error) {
					resp, err := httpmock.NewJsonResponse(http.StatusOK, map[string]interface{}{})
					Expect(err).ToNot(HaveOccurred())
					resp.Header.Set("Proxy-Connection", "keep-alive")
					resp.Header.Set("Keep-Alive", "timeout=5")
					resp.Header.Set("Transfer-Encoding", "chunked")
					resp.Header.Set("Te", "trailers")
					resp.Header.Set("Upgrade", "websocket")
					resp.Header.Set("X-TXID", "abc123")
					return resp, nil
				})

				req := httptest.NewRequest(http.MethodGet, "/api/1/unknown", nil)
				req.Header.Set("Authorization", authorizationToken)
				mockAccount.EXPECT().GetHost().Return("example.com")

				rr := httptest.NewRecorder()
				p.ServeHTTP(rr, req)

				Expect(rr.Code).To(Equal(http.StatusOK))
				Expect(rr.Header().Get("Proxy-Connection")).To(Equal(""))
				Expect(rr.Header().Get("Keep-Alive")).To(Equal(""))
				Expect(rr.Header().Get("Transfer-Encoding")).To(Equal(""))
				Expect(rr.Header().Get("Te")).To(Equal(""))
				Expect(rr.Header().Get("Upgrade")).To(Equal(""))
				Expect(rr.Header().Get("X-TXID")).To(Equal("abc123"))
			})
		})

		It("times out", func() {
			httpmock.Activate()
			defer httpmock.DeactivateAndReset()

			httpmock.RegisterResponder(http.MethodGet, "https://example.com/api/1/unknown", func(r *http.Request) (*http.Response, error) {
				time.Sleep(50 * time.Millisecond)
				return httpmock.NewJsonResponse(http.StatusOK, map[string]interface{}{})
			})

			p.Timeout = 25 * time.Millisecond
			req := httptest.NewRequest(http.MethodGet, "/api/1/unknown", nil)
			req.Header.Set("Authorization", authorizationToken)
			mockAccount.EXPECT().GetHost().Return("example.com")

			rr := httptest.NewRecorder()
			p.ServeHTTP(rr, req)

			Expect(rr.Code).To(Equal(http.StatusGatewayTimeout))
		})
	})

	It("returns 404 for path not starting with /api/1/", func() {
		req := httptest.NewRequest(http.MethodGet, "/unknown", nil)
		req.Header.Set("Authorization", authorizationToken)

		rr := httptest.NewRecorder()
		p.ServeHTTP(rr, req)

		Expect(rr.Code).To(Equal(http.StatusNotFound))
	})
})
