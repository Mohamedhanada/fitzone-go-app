package main

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type apiServer struct {
	db     *mongo.Database
	secret []byte
}
type apiUser struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name         string             `bson:"name" json:"name"`
	Email        string             `bson:"email" json:"email"`
	PasswordHash string             `bson:"passwordHash" json:"-"`
	CreatedAt    time.Time          `bson:"createdAt" json:"createdAt"`
}
type workoutPlan struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID    primitive.ObjectID `bson:"userId" json:"userId"`
	Name      string             `bson:"name" json:"name"`
	WeekStart string             `bson:"weekStart" json:"weekStart"`
	Days      []planDay          `bson:"days" json:"days"`
	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
}
type planDay struct {
	Day       string            `json:"day" bson:"day"`
	Exercises []plannedExercise `json:"exercises" bson:"exercises"`
}
type plannedExercise struct {
	ExerciseKey string `json:"exerciseKey" bson:"exerciseKey"`
	Sets        int    `json:"sets" bson:"sets"`
	Reps        int    `json:"reps" bson:"reps"`
	RestSeconds int    `json:"restSeconds" bson:"restSeconds"`
	Completed   bool   `json:"completed" bson:"completed"`
}
type progressEntry struct {
	ID       primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID   primitive.ObjectID `bson:"userId" json:"userId"`
	WeightKg float64            `bson:"weightKg" json:"weightKg"`
	Note     string             `bson:"note,omitempty" json:"note,omitempty"`
	Date     time.Time          `bson:"date" json:"date"`
}

func registerAPIRoutes(db *mongo.Database) {
	secret := os.Getenv("JWT_SECRET")
	if len(secret) < 32 {
		secret = "development-only-change-this-secret-before-deploying-123456"
		logWarningSecret()
	}
	s := &apiServer{db: db, secret: []byte(secret)}
	http.HandleFunc("/api/health", s.health)
	http.HandleFunc("/api/auth/register", s.register)
	http.HandleFunc("/api/auth/login", s.login)
	http.HandleFunc("/api/me", s.auth(s.me))
	http.HandleFunc("/api/uploads", s.auth(s.uploadImage))
	http.HandleFunc("/api/exercises", s.exercises)
	http.HandleFunc("/api/exercises/", s.exerciseByID)
	http.HandleFunc("/api/plans", s.auth(s.plans))
	http.HandleFunc("/api/plans/", s.auth(s.planByID))
	http.HandleFunc("/api/progress", s.auth(s.progress))
	http.HandleFunc("/api/progress/", s.auth(s.progressByID))
	_ = os.MkdirAll(filepath.Join("static", "uploads"), 0755)
}
func logWarningSecret() {
	fmt.Println("WARNING: JWT_SECRET is not set; using a development secret. Set JWT_SECRET in production.")
}
func (s *apiServer) health(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		method(w)
		return
	}
	jsonOut(w, 200, map[string]any{"status": "ok", "service": "fitzone-api"})
}
func (s *apiServer) register(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		method(w)
		return
	}
	var in struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	if in.Name == "" || !strings.Contains(in.Email, "@") || len(in.Password) < 8 || len(in.Password) > 200 {
		jsonOut(w, 400, map[string]string{"error": "name, valid email, and password (8+ characters) are required"})
		return
	}
	c := s.db.Collection("users")
	ctx, cancel := requestContext(r)
	defer cancel()
	n, err := c.CountDocuments(ctx, bson.M{"email": in.Email})
	if err != nil {
		serverError(w, err)
		return
	}
	if n > 0 {
		jsonOut(w, 409, map[string]string{"error": "email already registered"})
		return
	}
	hash, err := hashPassword(in.Password)
	if err != nil {
		serverError(w, err)
		return
	}
	u := apiUser{Name: in.Name, Email: in.Email, PasswordHash: hash, CreatedAt: time.Now().UTC()}
	res, err := c.InsertOne(ctx, u)
	if err != nil {
		serverError(w, err)
		return
	}
	u.ID = res.InsertedID.(primitive.ObjectID)
	token, err := s.token(u)
	if err != nil {
		serverError(w, err)
		return
	}
	jsonOut(w, 201, map[string]any{"user": u, "token": token})
}
func (s *apiServer) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		method(w)
		return
	}
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decode(w, r, &in) {
		return
	}
	ctx, cancel := requestContext(r)
	defer cancel()
	var u apiUser
	err := s.db.Collection("users").FindOne(ctx, bson.M{"email": strings.ToLower(strings.TrimSpace(in.Email))}).Decode(&u)
	if err != nil || !verifyPassword(in.Password, u.PasswordHash) {
		jsonOut(w, 401, map[string]string{"error": "invalid email or password"})
		return
	}
	t, err := s.token(u)
	if err != nil {
		serverError(w, err)
		return
	}
	jsonOut(w, 200, map[string]any{"user": u, "token": t})
}
func (s *apiServer) me(w http.ResponseWriter, r *http.Request, u apiUser) {
	if r.Method != "GET" {
		method(w)
		return
	}
	jsonOut(w, 200, map[string]any{"user": u})
}
func (s *apiServer) uploadImage(w http.ResponseWriter, r *http.Request, u apiUser) {
	if r.Method != "POST" {
		method(w)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 6<<20)
	if err := r.ParseMultipartForm(6 << 20); err != nil {
		jsonOut(w, 400, map[string]string{"error": "upload must be a multipart form under 6 MB"})
		return
	}
	f, h, err := r.FormFile("image")
	if err != nil {
		jsonOut(w, 400, map[string]string{"error": "attach image in form field named image"})
		return
	}
	defer f.Close()
	ext := strings.ToLower(filepath.Ext(h.Filename))
	allowed := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true}
	if !allowed[ext] {
		jsonOut(w, 400, map[string]string{"error": "only JPG, PNG, and WEBP images are allowed"})
		return
	}
	head := make([]byte, 512)
	n, _ := f.Read(head)
	contentType := http.DetectContentType(head[:n])
	if !strings.HasPrefix(contentType, "image/") {
		jsonOut(w, 400, map[string]string{"error": "uploaded file is not an image"})
		return
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		serverError(w, err)
		return
	}
	random := make([]byte, 16)
	if _, err = rand.Read(random); err != nil {
		serverError(w, err)
		return
	}
	name := hex.EncodeToString(random) + ext
	dst := filepath.Join("static", "uploads", name)
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		serverError(w, err)
		return
	}
	written, copyErr := io.Copy(out, io.LimitReader(f, 5<<20))
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(dst)
		serverError(w, fmt.Errorf("save upload: %v %v", copyErr, closeErr))
		return
	}
	jsonOut(w, 201, map[string]any{"image": "/static/uploads/" + name, "size": written, "contentType": contentType})
}
func (s *apiServer) exercises(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		ctx, cancel := requestContext(r)
		defer cancel()
		filter := bson.M{}
		if q := strings.TrimSpace(r.URL.Query().Get("q")); q != "" {
			filter = bson.M{"$or": bson.A{bson.M{"nameEN": bson.M{"$regex": q, "$options": "i"}}, bson.M{"nameAR": bson.M{"$regex": q}}, bson.M{"categoryEN": bson.M{"$regex": q, "$options": "i"}}, bson.M{"categoryAR": bson.M{"$regex": q}}}}
		}
		cur, err := s.db.Collection("exercises").Find(ctx, filter, options.Find().SetLimit(200))
		if err != nil {
			serverError(w, err)
			return
		}
		defer cur.Close(ctx)
		var out []bson.M
		if err = cur.All(ctx, &out); err != nil {
			serverError(w, err)
			return
		}
		if out == nil {
			out = []bson.M{}
		}
		jsonOut(w, 200, map[string]any{"exercises": out, "count": len(out)})
	case "POST":
		u, ok := s.requireUser(w, r)
		if !ok {
			return
		}
		_ = u
		var in bson.M
		if !decode(w, r, &in) {
			return
		}
		if strings.TrimSpace(fmt.Sprint(in["key"])) == "" {
			jsonOut(w, 400, map[string]string{"error": "key is required"})
			return
		}
		ctx, cancel := requestContext(r)
		defer cancel()
		if _, err := s.db.Collection("exercises").InsertOne(ctx, in); err != nil {
			serverError(w, err)
			return
		}
		jsonOut(w, 201, map[string]any{"exercise": in})
	default:
		method(w)
	}
}
func (s *apiServer) exerciseByID(w http.ResponseWriter, r *http.Request) {
	key := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/exercises/"), "/")
	if key == "" {
		http.NotFound(w, r)
		return
	}
	switch r.Method {
	case "GET":
		ctx, cancel := requestContext(r)
		defer cancel()
		var doc bson.M
		if err := s.db.Collection("exercises").FindOne(ctx, bson.M{"key": key}).Decode(&doc); err != nil {
			jsonOut(w, 404, map[string]string{"error": "exercise not found"})
			return
		}
		jsonOut(w, 200, map[string]any{"exercise": doc})
	case "PUT", "PATCH":
		if _, ok := s.requireUser(w, r); !ok {
			return
		}
		var in bson.M
		if !decode(w, r, &in) {
			return
		}
		delete(in, "_id")
		ctx, cancel := requestContext(r)
		defer cancel()
		res, err := s.db.Collection("exercises").UpdateOne(ctx, bson.M{"key": key}, bson.M{"$set": in})
		if err != nil {
			serverError(w, err)
			return
		}
		if res.MatchedCount == 0 {
			jsonOut(w, 404, map[string]string{"error": "exercise not found"})
			return
		}
		jsonOut(w, 200, map[string]string{"status": "updated"})
	case "DELETE":
		if _, ok := s.requireUser(w, r); !ok {
			return
		}
		ctx, cancel := requestContext(r)
		defer cancel()
		res, err := s.db.Collection("exercises").DeleteOne(ctx, bson.M{"key": key})
		if err != nil {
			serverError(w, err)
			return
		}
		if res.DeletedCount == 0 {
			jsonOut(w, 404, map[string]string{"error": "exercise not found"})
			return
		}
		jsonOut(w, 200, map[string]string{"status": "deleted"})
	default:
		method(w)
	}
}
func (s *apiServer) plans(w http.ResponseWriter, r *http.Request, u apiUser) {
	c := s.db.Collection("plans")
	ctx, cancel := requestContext(r)
	defer cancel()
	switch r.Method {
	case "GET":
		cur, err := c.Find(ctx, bson.M{"userId": u.ID}, options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
		if err != nil {
			serverError(w, err)
			return
		}
		defer cur.Close(ctx)
		var list []workoutPlan
		if err = cur.All(ctx, &list); err != nil {
			serverError(w, err)
			return
		}
		if list == nil {
			list = []workoutPlan{}
		}
		jsonOut(w, 200, map[string]any{"plans": list})
	case "POST":
		var p workoutPlan
		if !decode(w, r, &p) {
			return
		}
		if strings.TrimSpace(p.Name) == "" {
			jsonOut(w, 400, map[string]string{"error": "plan name is required"})
			return
		}
		p.ID = primitive.NewObjectID()
		p.UserID = u.ID
		p.CreatedAt = time.Now().UTC()
		if p.Days == nil {
			p.Days = []planDay{}
		}
		if _, err := c.InsertOne(ctx, p); err != nil {
			serverError(w, err)
			return
		}
		jsonOut(w, 201, map[string]any{"plan": p})
	default:
		method(w)
	}
}
func (s *apiServer) planByID(w http.ResponseWriter, r *http.Request, u apiUser) {
	id, err := primitive.ObjectIDFromHex(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/plans/"), "/"))
	if err != nil {
		jsonOut(w, 400, map[string]string{"error": "invalid plan id"})
		return
	}
	ctx, cancel := requestContext(r)
	defer cancel()
	c := s.db.Collection("plans")
	switch r.Method {
	case "GET":
		var p workoutPlan
		if err = c.FindOne(ctx, bson.M{"_id": id, "userId": u.ID}).Decode(&p); err != nil {
			jsonOut(w, 404, map[string]string{"error": "plan not found"})
			return
		}
		jsonOut(w, 200, map[string]any{"plan": p})
	case "PUT", "PATCH":
		var p workoutPlan
		if !decode(w, r, &p) {
			return
		}
		update := bson.M{"name": p.Name, "weekStart": p.WeekStart, "days": p.Days}
		res, e := c.UpdateOne(ctx, bson.M{"_id": id, "userId": u.ID}, bson.M{"$set": update})
		if e != nil {
			serverError(w, e)
			return
		}
		if res.MatchedCount == 0 {
			jsonOut(w, 404, map[string]string{"error": "plan not found"})
			return
		}
		jsonOut(w, 200, map[string]string{"status": "updated"})
	case "DELETE":
		res, e := c.DeleteOne(ctx, bson.M{"_id": id, "userId": u.ID})
		if e != nil {
			serverError(w, e)
			return
		}
		if res.DeletedCount == 0 {
			jsonOut(w, 404, map[string]string{"error": "plan not found"})
			return
		}
		jsonOut(w, 200, map[string]string{"status": "deleted"})
	default:
		method(w)
	}
}
func (s *apiServer) progress(w http.ResponseWriter, r *http.Request, u apiUser) {
	c := s.db.Collection("progress")
	ctx, cancel := requestContext(r)
	defer cancel()
	switch r.Method {
	case "GET":
		cur, err := c.Find(ctx, bson.M{"userId": u.ID}, options.Find().SetSort(bson.D{{Key: "date", Value: 1}}))
		if err != nil {
			serverError(w, err)
			return
		}
		defer cur.Close(ctx)
		var list []progressEntry
		if err = cur.All(ctx, &list); err != nil {
			serverError(w, err)
			return
		}
		if list == nil {
			list = []progressEntry{}
		}
		jsonOut(w, 200, map[string]any{"entries": list})
	case "POST":
		var p progressEntry
		if !decode(w, r, &p) {
			return
		}
		if p.WeightKg < 20 || p.WeightKg > 500 {
			jsonOut(w, 400, map[string]string{"error": "weightKg must be between 20 and 500"})
			return
		}
		if p.Date.IsZero() {
			p.Date = time.Now().UTC()
		} else {
			p.Date = p.Date.UTC()
		}
		p.ID = primitive.NewObjectID()
		p.UserID = u.ID
		if _, err := c.InsertOne(ctx, p); err != nil {
			serverError(w, err)
			return
		}
		jsonOut(w, 201, map[string]any{"entry": p})
	default:
		method(w)
	}
}
func (s *apiServer) progressByID(w http.ResponseWriter, r *http.Request, u apiUser) {
	if r.Method != "DELETE" {
		method(w)
		return
	}
	id, err := primitive.ObjectIDFromHex(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/progress/"), "/"))
	if err != nil {
		jsonOut(w, 400, map[string]string{"error": "invalid progress id"})
		return
	}
	ctx, cancel := requestContext(r)
	defer cancel()
	res, err := s.db.Collection("progress").DeleteOne(ctx, bson.M{"_id": id, "userId": u.ID})
	if err != nil {
		serverError(w, err)
		return
	}
	if res.DeletedCount == 0 {
		jsonOut(w, 404, map[string]string{"error": "entry not found"})
		return
	}
	jsonOut(w, 200, map[string]string{"status": "deleted"})
}
func (s *apiServer) auth(next func(http.ResponseWriter, *http.Request, apiUser)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, ok := s.requireUser(w, r)
		if !ok {
			return
		}
		next(w, r, u)
	}
}
func (s *apiServer) requireUser(w http.ResponseWriter, r *http.Request) (apiUser, bool) {
	h := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(h, "Bearer ") {
		jsonOut(w, 401, map[string]string{"error": "Bearer token required"})
		return apiUser{}, false
	}
	id, err := s.parseToken(strings.TrimPrefix(h, "Bearer "))
	if err != nil {
		jsonOut(w, 401, map[string]string{"error": "invalid or expired token"})
		return apiUser{}, false
	}
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		jsonOut(w, 401, map[string]string{"error": "invalid token subject"})
		return apiUser{}, false
	}
	ctx, cancel := requestContext(r)
	defer cancel()
	var u apiUser
	if err = s.db.Collection("users").FindOne(ctx, bson.M{"_id": oid}).Decode(&u); err != nil {
		jsonOut(w, 401, map[string]string{"error": "user not found"})
		return apiUser{}, false
	}
	return u, true
}
func (s *apiServer) token(u apiUser) (string, error) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payloadBytes, err := json.Marshal(map[string]any{"sub": u.ID.Hex(), "email": u.Email, "exp": time.Now().Add(24 * time.Hour).Unix()})
	if err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	body := header + "." + payload
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(body))
	return body + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}
func (s *apiServer) parseToken(token string) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", errors.New("malformed token")
	}
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(parts[0] + "." + parts[1]))
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(sig, mac.Sum(nil)) {
		return "", errors.New("bad signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", err
	}
	var claims struct {
		Sub string `json:"sub"`
		Exp int64  `json:"exp"`
	}
	if err = json.Unmarshal(payload, &claims); err != nil {
		return "", err
	}
	if claims.Sub == "" || time.Now().Unix() >= claims.Exp {
		return "", errors.New("expired")
	}
	return claims.Sub, nil
}
func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	sum := derive(password, salt, 120000)
	return "pbkdf2-sha256$120000$" + base64.RawStdEncoding.EncodeToString(salt) + "$" + base64.RawStdEncoding.EncodeToString(sum), nil
}
func verifyPassword(password, encoded string) bool {
	p := strings.Split(encoded, "$")
	if len(p) != 4 || p[0] != "pbkdf2-sha256" {
		return false
	}
	var rounds int
	if _, err := fmt.Sscanf(p[1], "%d", &rounds); err != nil || rounds < 1 || rounds > 1000000 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(p[2])
	if err != nil {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(p[3])
	if err != nil {
		return false
	}
	return hmac.Equal(derive(password, salt, rounds), expected)
}
func derive(password string, salt []byte, rounds int) []byte { // PBKDF2-HMAC-SHA256, one 32-byte block
	mac := hmac.New(sha256.New, []byte(password))
	mac.Write(salt)
	mac.Write([]byte{0, 0, 0, 1})
	u := mac.Sum(nil)
	out := append([]byte(nil), u...)
	for i := 1; i < rounds; i++ {
		mac = hmac.New(sha256.New, []byte(password))
		mac.Write(u)
		u = mac.Sum(nil)
		for j := range out {
			out[j] ^= u[j]
		}
	}
	return out
}
func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		jsonOut(w, 400, map[string]string{"error": "invalid JSON body: " + err.Error()})
		return false
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		jsonOut(w, 400, map[string]string{"error": "request must contain one JSON object"})
		return false
	}
	return true
}
func jsonOut(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func method(w http.ResponseWriter) {
	w.Header().Set("Allow", "GET, POST, PUT, PATCH, DELETE")
	jsonOut(w, 405, map[string]string{"error": "method not allowed"})
}
func serverError(w http.ResponseWriter, err error) {
	fmt.Printf("API error: %v\n", err)
	jsonOut(w, 500, map[string]string{"error": "internal server error"})
}
func requestContext(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), 8*time.Second)
}
func _unusedHex(s string) string { return hex.EncodeToString([]byte(s)) }
