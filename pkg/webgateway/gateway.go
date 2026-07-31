// Package webgateway exposes the VectorRouter gRPC surface as HTTP/JSON for the Master web prototype.
package webgateway

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	vectorv1 "github.com/vectorial-dua/avlp/gen/avlp/vector/v1"
	"github.com/vectorial-dua/avlp/pkg/session"
	"github.com/vectorial-dua/avlp/pkg/stt"
)

// StudentUnavailableMessage is the person-centered copy for transport / 5xx failures.
const StudentUnavailableMessage = "No pudimos conectar con el tutor en este momento; probá de nuevo en un instante"

const promoteDeniedMessage = "No tenés permiso para esta acción."

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
	Client      vectorv1.VectorRouterClient
	Static      http.Handler // optional; served for non-/api/ paths
	Session     session.Config
	Transcriber *stt.HTTPTranscriber // optional local STT
	Now         func() time.Time
}

const (
	sttUnavailableMessage = "El dictado local no está disponible en este servidor."
	sttFailedMessage      = "No pude transcribir el audio. Probá de nuevo o escribí la duda."
	sttTooLargeMessage    = "El audio es demasiado largo. Grabá menos de un minuto o escribí la duda."
	sttEmptyMessage       = "No recibí audio para transcribir. Probá de nuevo."
)

// New returns a gateway. Static may be nil. Session defaults to env when zero.
func New(client vectorv1.VectorRouterClient, static http.Handler) *Gateway {
	return &Gateway{
		Client:  client,
		Static:  static,
		Session: session.FromEnv(),
		Now:     func() time.Time { return time.Now().UTC() },
	}
}

// Handler returns the root HTTP handler.
func (g *Gateway) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/session", g.handleSession)
	mux.HandleFunc("POST /api/transcribe", g.handleTranscribe)
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
			w.Header().Set("Cache-Control", "no-cache")
			g.Static.ServeHTTP(w, r)
			return
		}
		http.NotFound(w, r)
	})
}

type sessionRequest struct {
	StudentID  string `json:"student_id"`
	TeacherKey string `json:"teacher_key"`
}

type sessionResponse struct {
	StudentID  string `json:"student_id"`
	Token      string `json:"token"`
	Role       string `json:"role"`
	SecureMode bool   `json:"secure_mode"`
	STTEnabled bool   `json:"stt_enabled"`
	ExpiresAt  int64  `json:"expires_at,omitempty"`
}

func (g *Gateway) handleSession(w http.ResponseWriter, r *http.Request) {
	var body sessionRequest
	if err := decodeJSON(r, &body); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_argument", err.Error(), "")
		return
	}
	token, claims, err := g.Session.Issue(body.StudentID, body.TeacherKey, g.now())
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_argument", err.Error(), "")
		return
	}
	writeJSON(w, http.StatusOK, sessionResponse{
		StudentID:  claims.StudentID,
		Token:      token,
		Role:       claims.Role,
		SecureMode: g.Session.Secure(),
		STTEnabled: g.Transcriber != nil,
		ExpiresAt:  claims.Exp,
	})
}

func (g *Gateway) handleTranscribe(w http.ResponseWriter, r *http.Request) {
	if _, _, err := g.authedContext(r, ""); err != nil {
		writeAuthError(w, err)
		return
	}
	if g.Transcriber == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "unavailable", sttUnavailableMessage, sttUnavailableMessage)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, stt.MaxAudioBytes+1<<20)
	if err := r.ParseMultipartForm(stt.MaxAudioBytes + 1<<20); err != nil {
		msg := "No pude leer el audio. Probá de nuevo o escribí la duda."
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) || strings.Contains(strings.ToLower(err.Error()), "too large") {
			msg = sttTooLargeMessage
		}
		writeAPIError(w, http.StatusBadRequest, "invalid_argument", msg, msg)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_argument", sttEmptyMessage, sttEmptyMessage)
		return
	}
	defer file.Close()
	audio, err := io.ReadAll(io.LimitReader(file, stt.MaxAudioBytes+1))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_argument", sttFailedMessage, sttFailedMessage)
		return
	}
	if len(audio) == 0 {
		writeAPIError(w, http.StatusBadRequest, "invalid_argument", sttEmptyMessage, sttEmptyMessage)
		return
	}
	if len(audio) > stt.MaxAudioBytes {
		writeAPIError(w, http.StatusBadRequest, "invalid_argument", sttTooLargeMessage, sttTooLargeMessage)
		return
	}
	filename := "audio.webm"
	contentType := "application/octet-stream"
	if header != nil {
		if name := strings.TrimSpace(header.Filename); name != "" {
			filename = name
		}
		if ct := strings.TrimSpace(header.Header.Get("Content-Type")); ct != "" {
			contentType = ct
		}
	}
	text, err := g.Transcriber.Transcribe(r.Context(), audio, filename, contentType)
	if err != nil {
		log.Printf("webgateway transcribe: %v", err)
		writeAPIError(w, http.StatusBadGateway, "unavailable", sttFailedMessage, sttFailedMessage)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"text": text})
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
	ctx, id, err := g.authedContext(r, body.StudentID)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	studentID, err := session.ResolveStudentID(id, body.StudentID)
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	if strings.TrimSpace(body.QueryText) == "" {
		writeAPIError(w, http.StatusBadRequest, "invalid_argument", "query_text is required", "")
		return
	}

	st := &vectorv1.StudentVector{
		StudentId: studentID,
		Session:   &vectorv1.StudentSessionMeta{},
	}
	if body.Frustration != nil {
		st.Session.FrustrationSignal = body.Frustration
	}

	res, err := g.Client.QueryNearestNode(ctx, &vectorv1.VectorQuery{
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
	ctx, _, err := g.authedContext(r, "")
	if err != nil {
		writeAuthError(w, err)
		return
	}
	res, err := g.Client.GetInteractiveNode(ctx, &vectorv1.NodeIdRequest{NodeId: id})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeProto(w, http.StatusOK, res)
}

func (g *Gateway) handleSubtopicProgress(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	requested := r.URL.Query().Get("student_id")
	if id == "" {
		writeAPIError(w, http.StatusBadRequest, "invalid_argument", "node id is required", "")
		return
	}
	ctx, identity, err := g.authedContext(r, requested)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	studentID, err := session.ResolveStudentID(identity, requested)
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	res, err := g.Client.GetSubtopicProgress(ctx, &vectorv1.SubtopicProgressQuery{
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
	ctx, identity, err := g.authedContext(r, body.StudentID)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	studentID, err := session.ResolveStudentID(identity, body.StudentID)
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	if id == "" || strings.TrimSpace(body.DoubtText) == "" {
		writeAPIError(w, http.StatusBadRequest, "invalid_argument", "node id and doubt_text are required", "")
		return
	}
	res, err := g.Client.MutateInteractiveNode(ctx, &vectorv1.MutateInteractiveRequest{
		NodeId:      id,
		StudentId:   studentID,
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
	ctx, identity, err := g.authedContext(r, req.GetStudentId())
	if err != nil {
		writeAuthError(w, err)
		return
	}
	studentID, err := session.ResolveStudentID(identity, req.GetStudentId())
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	req.StudentId = studentID
	res, err := g.Client.RecordBotoneraInteraction(ctx, &req)
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
	ctx, identity, err := g.authedContext(r, req.GetStudentId())
	if err != nil {
		writeAuthError(w, err)
		return
	}
	studentID, err := session.ResolveStudentID(identity, req.GetStudentId())
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	req.StudentId = studentID
	res, err := g.Client.RecordSubtopicInteraction(ctx, &req)
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeProto(w, http.StatusOK, res)
}

func (g *Gateway) handleStation(w http.ResponseWriter, r *http.Request) {
	ulid := r.PathValue("tracking_ulid")
	requested := r.URL.Query().Get("student_id")
	if ulid == "" {
		writeAPIError(w, http.StatusBadRequest, "invalid_argument", "tracking_ulid is required", "")
		return
	}
	ctx, identity, err := g.authedContext(r, requested)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	studentID, err := session.ResolveStudentID(identity, requested)
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	res, err := g.Client.GetLiveStation(ctx, &vectorv1.LiveStationQuery{
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
	ctx, identity, err := g.authedContext(r, "")
	if err != nil {
		writeAuthError(w, err)
		return
	}
	if identity.Secure && identity.Role != session.RoleTeacher {
		writeAPIError(w, http.StatusForbidden, "permission_denied", promoteDeniedMessage, promoteDeniedMessage)
		return
	}
	res, err := g.Client.PromoteLiveStation(ctx, &vectorv1.PromoteLiveStationRequest{
		TrackingUlid: ulid,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeProto(w, http.StatusOK, res)
}

func (g *Gateway) now() time.Time {
	if g.Now != nil {
		return g.Now()
	}
	return time.Now().UTC()
}

// authedContext validates Bearer when secure and attaches outgoing gRPC metadata.
// requestedStudentID is optional hint used only when issuing open-mode identity.
func (g *Gateway) authedContext(r *http.Request, requestedStudentID string) (context.Context, session.Identity, error) {
	if !g.Session.Secure() {
		id := session.Identity{
			StudentID: strings.TrimSpace(requestedStudentID),
			Role:      session.RoleStudent,
			Secure:    false,
		}
		return session.AppendOutgoingMetadata(r.Context(), id), id, nil
	}
	raw := strings.TrimSpace(r.Header.Get("Authorization"))
	if raw == "" || !strings.HasPrefix(strings.ToLower(raw), "bearer ") {
		return nil, session.Identity{}, status.Error(codes.Unauthenticated, "authentication required")
	}
	token := strings.TrimSpace(raw[len("Bearer "):])
	claims, err := g.Session.Verify(token, g.now())
	if err != nil {
		if errors.Is(err, session.ErrExpiredToken) {
			return nil, session.Identity{}, status.Error(codes.Unauthenticated, "session expired")
		}
		return nil, session.Identity{}, status.Error(codes.Unauthenticated, "authentication required")
	}
	id := session.Identity{
		StudentID: claims.StudentID,
		Role:      claims.Role,
		Secure:    true,
	}
	return session.AppendOutgoingMetadata(r.Context(), id), id, nil
}

func writeAuthError(w http.ResponseWriter, err error) {
	writeGRPCError(w, err)
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

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
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
	case codes.PermissionDenied:
		return promoteDeniedMessage
	case codes.Unauthenticated:
		return "Necesitás una sesión activa para continuar."
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
	switch st.Code() {
	case codes.NotFound:
		studentMsg = raw
		msg = raw
	case codes.PermissionDenied, codes.Unauthenticated:
		studentMsg = msg
	case codes.Unavailable, codes.Internal, codes.DeadlineExceeded, codes.Unknown:
		studentMsg = StudentUnavailableMessage
	default:
		if msg == StudentUnavailableMessage {
			studentMsg = StudentUnavailableMessage
		}
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
	case codes.Unauthenticated:
		return http.StatusUnauthorized, "unauthenticated"
	case codes.Internal:
		return http.StatusInternalServerError, "internal"
	default:
		return http.StatusBadGateway, "bad_gateway"
	}
}
