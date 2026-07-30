// Package webgateway exposes the VectorRouter gRPC surface as HTTP/JSON for the Master web prototype.
package webgateway

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	vectorv1 "github.com/vectorial-dua/avlp/gen/avlp/vector/v1"
)

// StudentUnavailableMessage is the person-centered copy for transport / 5xx failures.
const StudentUnavailableMessage = "No pudimos conectar con el tutor en este momento; probá de nuevo en un instante"

var (
	marshalOpts = protojson.MarshalOptions{
		UseProtoNames:   true,
		EmitUnpopulated: true,
	}
	unmarshalOpts = protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}
)

// Gateway maps REST/JSON routes onto a VectorRouterClient.
type Gateway struct {
	Client vectorv1.VectorRouterClient
	Static http.Handler // optional; served for non-/api/ paths
}

// New returns a gateway. Static may be nil.
func New(client vectorv1.VectorRouterClient, static http.Handler) *Gateway {
	return &Gateway{Client: client, Static: static}
}

// Handler returns the root HTTP handler.
func (g *Gateway) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/query", g.handleQuery)
	mux.HandleFunc("GET /api/nodes/{id}", g.handleGetNode)
	mux.HandleFunc("GET /api/nodes/{id}/progress", g.handleSubtopicProgress)
	mux.HandleFunc("POST /api/nodes/{id}/mutate", g.handleMutate)
	mux.HandleFunc("POST /api/interactions/botonera", g.handleBotonera)
	mux.HandleFunc("POST /api/interactions/subtopic", g.handleSubtopic)
	mux.HandleFunc("GET /api/stations/{tracking_ulid}", g.handleStation)
	mux.HandleFunc("POST /api/stations/{tracking_ulid}/promote", g.handlePromoteStation)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			mux.ServeHTTP(w, r)
			return
		}
		if g.Static != nil {
			// El shell y su grafo de módulos deben revalidarse juntos para evitar
			// mezclar archivos de despliegues distintos.
			w.Header().Set("Cache-Control", "no-cache")
			g.Static.ServeHTTP(w, r)
			return
		}
		http.NotFound(w, r)
	})
}

type queryRequest struct {
	StudentID   string   `json:"student_id"`
	QueryText   string   `json:"query_text"`
	Frustration *float32 `json:"frustration"`
}

func (g *Gateway) handleQuery(w http.ResponseWriter, r *http.Request) {
	var body queryRequest
	if err := decodeJSON(r, &body); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_argument", err.Error(), "")
		return
	}
	if body.StudentID == "" || strings.TrimSpace(body.QueryText) == "" {
		writeAPIError(w, http.StatusBadRequest, "invalid_argument", "student_id and query_text are required", "")
		return
	}

	st := &vectorv1.StudentVector{
		StudentId: body.StudentID,
		Session:   &vectorv1.StudentSessionMeta{},
	}
	if body.Frustration != nil {
		st.Session.FrustrationSignal = body.Frustration
	}

	res, err := g.Client.QueryNearestNode(r.Context(), &vectorv1.VectorQuery{
		StudentState: st,
		QueryText:    body.QueryText,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeProto(w, http.StatusOK, res)
}

func (g *Gateway) handleGetNode(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeAPIError(w, http.StatusBadRequest, "invalid_argument", "node id is required", "")
		return
	}
	res, err := g.Client.GetInteractiveNode(r.Context(), &vectorv1.NodeIdRequest{NodeId: id})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeProto(w, http.StatusOK, res)
}

func (g *Gateway) handleSubtopicProgress(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	studentID := r.URL.Query().Get("student_id")
	if id == "" || studentID == "" {
		writeAPIError(w, http.StatusBadRequest, "invalid_argument", "node id and student_id are required", "")
		return
	}
	res, err := g.Client.GetSubtopicProgress(r.Context(), &vectorv1.SubtopicProgressQuery{
		StudentId:    studentID,
		ParentNodeId: id,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeProto(w, http.StatusOK, res)
}

type mutateRequest struct {
	StudentID   string  `json:"student_id"`
	DoubtText   string  `json:"doubt_text"`
	Frustration float32 `json:"frustration"`
}

func (g *Gateway) handleMutate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body mutateRequest
	if err := decodeJSON(r, &body); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_argument", err.Error(), "")
		return
	}
	if id == "" || body.StudentID == "" || strings.TrimSpace(body.DoubtText) == "" {
		writeAPIError(w, http.StatusBadRequest, "invalid_argument", "node id, student_id, and doubt_text are required", "")
		return
	}
	res, err := g.Client.MutateInteractiveNode(r.Context(), &vectorv1.MutateInteractiveRequest{
		NodeId:      id,
		StudentId:   body.StudentID,
		DoubtText:   body.DoubtText,
		Frustration: body.Frustration,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeProto(w, http.StatusOK, res)
}

func (g *Gateway) handleBotonera(w http.ResponseWriter, r *http.Request) {
	var req vectorv1.BotoneraInteraction
	if err := decodeProtoJSON(r, &req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_argument", err.Error(), "")
		return
	}
	res, err := g.Client.RecordBotoneraInteraction(r.Context(), &req)
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeProto(w, http.StatusOK, res)
}

func (g *Gateway) handleSubtopic(w http.ResponseWriter, r *http.Request) {
	var req vectorv1.SubtopicInteraction
	if err := decodeProtoJSON(r, &req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_argument", err.Error(), "")
		return
	}
	res, err := g.Client.RecordSubtopicInteraction(r.Context(), &req)
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeProto(w, http.StatusOK, res)
}

func (g *Gateway) handleStation(w http.ResponseWriter, r *http.Request) {
	ulid := r.PathValue("tracking_ulid")
	studentID := r.URL.Query().Get("student_id")
	if ulid == "" || studentID == "" {
		writeAPIError(w, http.StatusBadRequest, "invalid_argument", "tracking_ulid and student_id are required", "")
		return
	}
	res, err := g.Client.GetLiveStation(r.Context(), &vectorv1.LiveStationQuery{
		TrackingUlid: ulid,
		StudentId:    studentID,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeProto(w, http.StatusOK, res)
}

func (g *Gateway) handlePromoteStation(w http.ResponseWriter, r *http.Request) {
	ulid := r.PathValue("tracking_ulid")
	if ulid == "" {
		writeAPIError(w, http.StatusBadRequest, "invalid_argument", "tracking_ulid is required", "")
		return
	}
	res, err := g.Client.PromoteLiveStation(r.Context(), &vectorv1.PromoteLiveStationRequest{
		TrackingUlid: ulid,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeProto(w, http.StatusOK, res)
}

func decodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	return dec.Decode(dst)
}

func decodeProtoJSON(r *http.Request, msg proto.Message) error {
	defer r.Body.Close()
	b, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return err
	}
	return unmarshalOpts.Unmarshal(b, msg)
}

func writeProto(w http.ResponseWriter, code int, msg proto.Message) {
	b, err := marshalOpts.Marshal(msg)
	if err != nil {
		log.Printf("webgateway marshal: %v", err)
		writeAPIError(w, http.StatusInternalServerError, "internal", StudentUnavailableMessage, "")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_, _ = w.Write(b)
}

type apiError struct {
	Code           string `json:"code"`
	Message        string `json:"message"`
	StudentMessage string `json:"student_message,omitempty"`
}

func writeAPIError(w http.ResponseWriter, httpCode int, code, message, studentMessage string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(httpCode)
	_ = json.NewEncoder(w).Encode(apiError{
		Code:           code,
		Message:        message,
		StudentMessage: studentMessage,
	})
}

func isTechnicalTransportMessage(msg string) bool {
	m := strings.ToLower(msg)
	return strings.Contains(m, "dial tcp") ||
		strings.Contains(m, "transport:") ||
		strings.Contains(m, "connection error") ||
		strings.Contains(m, "connection refused") ||
		strings.Contains(m, "unavailable") && strings.Contains(m, "rpc")
}

func studentFacingGRPCMessage(code codes.Code, raw string) string {
	switch code {
	case codes.Unavailable, codes.Internal, codes.DeadlineExceeded, codes.Unknown:
		return StudentUnavailableMessage
	}
	if isTechnicalTransportMessage(raw) {
		return StudentUnavailableMessage
	}
	return raw
}

func writeGRPCError(w http.ResponseWriter, err error) {
	st, ok := status.FromError(err)
	if !ok {
		log.Printf("webgateway transport: %v", err)
		writeAPIError(w, http.StatusBadGateway, "unavailable", StudentUnavailableMessage, StudentUnavailableMessage)
		return
	}
	raw := st.Message()
	httpCode, code := mapGRPCCode(st.Code())
	msg := studentFacingGRPCMessage(st.Code(), raw)
	if msg != raw {
		log.Printf("webgateway grpc %s (hidden from student): %v", st.Code(), err)
	}
	studentMsg := ""
	if st.Code() == codes.NotFound {
		studentMsg = raw // rogerian / NotFound copy is already student-facing
		msg = raw
	} else if msg == StudentUnavailableMessage {
		studentMsg = StudentUnavailableMessage
	}
	writeAPIError(w, httpCode, code, msg, studentMsg)
}

func mapGRPCCode(c codes.Code) (httpCode int, code string) {
	switch c {
	case codes.NotFound:
		return http.StatusNotFound, "not_found"
	case codes.InvalidArgument:
		return http.StatusBadRequest, "invalid_argument"
	case codes.FailedPrecondition:
		return http.StatusPreconditionFailed, "failed_precondition"
	case codes.Unimplemented:
		return http.StatusNotImplemented, "unimplemented"
	case codes.Unavailable:
		return http.StatusServiceUnavailable, "unavailable"
	case codes.DeadlineExceeded:
		return http.StatusGatewayTimeout, "deadline_exceeded"
	case codes.PermissionDenied:
		return http.StatusForbidden, "permission_denied"
	case codes.Internal:
		return http.StatusInternalServerError, "internal"
	default:
		return http.StatusBadGateway, "bad_gateway"
	}
}
