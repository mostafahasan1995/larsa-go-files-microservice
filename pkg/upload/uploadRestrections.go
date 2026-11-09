package upload

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"

	"larsa-files/domain"

	"git.larsa.io/mahdawi/microservices-commons.git/common"
)

type Restrictions struct {
	RequestFile bool
	Limits      Limits
	err         error
	isEmpty     bool
}

type fileSize struct {
	Value int
	Unit  string
}

type Limits struct {
	FileFormats []string
	FileMaxSize fileSize
}

func UploadRestructionsMiddleware() common.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {

			h := common.ExtractHeaderParams(r)
			service := h.Service
			if service == "" {
				res := Restrictions{isEmpty: true}
				ctx := context.WithValue(r.Context(), common.CommonContextKey("uploadRestructions"), res)
				next.ServeHTTP(rw, r.Clone(ctx))
				return
			}
			query := r.URL.Query().Get("query")

			var res Restrictions
			var reqopt *common.RequestParams = &common.RequestParams{
				Service: service,
				Path:    "_files_config?query=" + query,
				Method:  "GET",
				Data:    map[string]interface{}{},
				Header:  h,
			}

			resp, err := common.CallService(reqopt)
			if err != nil {
				res := Restrictions{isEmpty: true}
				ctx := context.WithValue(r.Context(), common.CommonContextKey("uploadRestructions"), res)
				next.ServeHTTP(rw, r.Clone(ctx))
				return
			}

			if resp.StatusCode == 200 {
				json.NewDecoder(resp.Body).Decode(&res)

				if res.RequestFile {
					// send file to the service
					pipr, pipw := io.Pipe()

					reader, err := r.MultipartReader()
					if err != nil {
						res := Restrictions{err: err}
						ctx := context.WithValue(r.Context(), common.CommonContextKey("uploadRestructions"), res)
						next.ServeHTTP(rw, r.Clone(ctx))
						return
					}

					mpw := multipart.NewWriter(pipw)

					errchan := make(chan error)

					go func() {
						defer close(errchan)
						defer pipw.Close()
						defer mpw.Close()

						for {
							part, errPart := reader.NextPart()
							if errPart == io.EOF {
								break
							}

							h := make(textproto.MIMEHeader)
							h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s";  filename="%s"`, part.FormName(), part.FileName()))
							h.Set("Content-Type", part.Header.Get("Content-Type"))
							spart, err := mpw.CreatePart(h)

							if err != nil {
								errchan <- err
								break
							}

							_, err = io.Copy(spart, part)
							if err != nil {
								errchan <- err
								break
							}
						}

						if err := mpw.Close(); err != nil {
							errchan <- err
							return
						}
					}()

					//upload request
					req, err := http.NewRequest("POST", common.GetServiceUrl(h.Service)+"/_files_upload?query="+query, pipr)
					if err != nil {
						http.Error(rw, err.Error(), http.StatusBadRequest)
						return
					}
					req.Header.Add("x-client", h.Client)
					req.Header.Add("x-service", h.Service)
					req.Header.Add("x-service-token", h.ServiceToken)
					req.Header.Add("x-user-id", h.UserId)
					req.Header.Add("x-access-token", h.AccessToken)
					req.Header.Add("accept-language", h.AcceptLanguage)
					req.Header.Set("Content-Type", mpw.FormDataContentType())

					client := &http.Client{}
					resp, errResp := client.Do(req)

					merr := <-errchan
					if merr != nil {
						http.Error(rw, err.Error(), http.StatusBadRequest)
						return
					}
					if errResp != nil {
						http.Error(rw, err.Error(), http.StatusBadRequest)
						return
					}
					defer resp.Body.Close()
					//End upload request

					var resFile domain.File
					err = json.NewDecoder(resp.Body).Decode(&resFile)
					if err != nil {
						http.Error(rw, err.Error(), http.StatusBadRequest)
						return
					}

					if resp.StatusCode != 200 {
						// s, err := ioutil.ReadAll(resp.Body)
						// if err != nil {
						// 	fmt.Println("read err", err.Error())
						// }
						// fmt.Println(string(s))
						http.Error(rw, "resturctions not satisfied by service "+h.Service, http.StatusBadRequest)
						return
					} else {
						json.NewEncoder(rw).Encode(resFile)
						return
					}
				} else {
					ctx := context.WithValue(r.Context(), common.CommonContextKey("uploadRestructions"), res)
					next.ServeHTTP(rw, r.Clone(ctx))
					return
				}
			} else {
				res := Restrictions{isEmpty: true}
				ctx := context.WithValue(r.Context(), common.CommonContextKey("uploadRestructions"), res)
				next.ServeHTTP(rw, r.Clone(ctx))
				return
			}
		})
	}
}

func EnablePublicUploadMiddleware() common.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			headerParams := common.ExtractHeaderParams(r)

			isEnable, err := common.GetOptionValue("ENABLE_PUBLIC_UPLOAD", headerParams)
			if err != nil {
				http.Error(rw, "", http.StatusMethodNotAllowed)
				return
			}

			isEnableBool, ok := isEnable.(bool)
			if !ok {
				http.Error(rw, "", http.StatusMethodNotAllowed)
				return
			}

			if !isEnableBool {
				http.Error(rw, "", http.StatusMethodNotAllowed)
				return
			}

			next.ServeHTTP(rw, r)

		})
	}
}
