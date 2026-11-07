package main

import (
	"context"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Exercise struct {
	Name, Category, Level, Sets, Description, Image string
	Key                                             string
}
type PageData struct {
	Title, Lang, Search, Category string
	IsEnglish                     bool
	Exercises                     []Exercise
	Categories                    []Category
}
type Category struct{ Key, Label string }

type exerciseText struct {
	Key         string `bson:"key" json:"key"`
	NameAR      string `bson:"nameAR" json:"nameAR"`
	NameEN      string `bson:"nameEN" json:"nameEN"`
	CategoryKey string `bson:"categoryKey" json:"categoryKey"`
	CategoryAR  string `bson:"categoryAR" json:"categoryAR"`
	CategoryEN  string `bson:"categoryEN" json:"categoryEN"`
	LevelAR     string `bson:"levelAR" json:"levelAR"`
	LevelEN     string `bson:"levelEN" json:"levelEN"`
	SetsAR      string `bson:"setsAR" json:"setsAR"`
	SetsEN      string `bson:"setsEN" json:"setsEN"`
	DescAR      string `bson:"descAR" json:"descAR"`
	DescEN      string `bson:"descEN" json:"descEN"`
	Image       string `bson:"image" json:"image"`
}

var allExercises = []exerciseText{
	{"pushups", "ضغط الصدر (Push-ups)", "Push-ups", "chest", "الصدر", "Chest", "مبتدئ", "Beginner", "3 مجموعات × 10–15 تكرار", "3 sets × 10–15 reps", "ضع يديك أسفل الكتفين، وانزل بجسمك بتحكم ثم ادفع للأعلى مع الحفاظ على استقامة الجسم.", "Place your hands under your shoulders, lower with control, then push up while keeping your body straight.", "pushups.jpg"},
	{"squats", "القرفصاء (Squats)", "Squats", "legs", "الأرجل", "Legs", "مبتدئ", "Beginner", "3 مجموعات × 12–15 تكرار", "3 sets × 12–15 reps", "ادفع الحوض للخلف واثنِ الركبتين مع الحفاظ على الظهر في وضع محايد.", "Push your hips back and bend your knees while keeping your back neutral.", "squats.jpg"},
	{"plank", "البلانك (Plank)", "Plank", "abs", "البطن", "Abs", "مبتدئ", "Beginner", "3 مجموعات × 30–60 ثانية", "3 sets × 30–60 sec", "حافظ على جسمك في خط مستقيم وشد عضلات البطن دون حبس النفس.", "Keep your body in a straight line and brace your core without holding your breath.", "plank.jpg"},
	{"pullups", "السحب لأعلى (Pull-ups)", "Pull-ups", "back", "الظهر", "Back", "متوسط", "Intermediate", "3 مجموعات × 6–12 تكرار", "3 sets × 6–12 reps", "امسك العقلة واسحب جسمك بتحكم. استخدم مساعدة إذا كنت مبتدئًا.", "Grip the bar and pull up with control. Use assistance if you are a beginner.", "pullups.jpg"},
	{"shoulderpress", "ضغط الكتف (Shoulder Press)", "Shoulder Press", "shoulders", "الأكتاف", "Shoulders", "متوسط", "Intermediate", "3 مجموعات × 10–12 تكرار", "3 sets × 10–12 reps", "ادفع الأوزان لأعلى دون تقويس أسفل الظهر، واختر وزنًا مناسبًا.", "Press the weights overhead without arching your lower back; use a manageable weight.", "shoulder_press.jpg"},
	{"lunges", "الاندفاع الأمامي (Lunges)", "Forward Lunges", "legs", "الأرجل", "Legs", "مبتدئ", "Beginner", "3 مجموعات × 10 لكل رجل", "3 sets × 10 each leg", "خذ خطوة للأمام وانزل بتحكم، ثم ارجع لوضع البداية وبدّل الرجل.", "Step forward and lower with control, return to the start, then switch legs.", "lunges.jpg"},
	{"biceps", "تمرين البايسبس بالدمبل", "Dumbbell Biceps Curl", "arms", "الذراعان", "Arms", "مبتدئ", "Beginner", "3 مجموعات × 10–12 تكرار", "3 sets × 10–12 reps", "ثبت المرفقين بجانب جسمك وارفع الدمبل دون تأرجح.", "Keep your elbows close to your body and curl without swinging.", "biceps_curl.jpg"},
	{"kneeraises", "رفع الركبتين", "Knee Raises", "abs", "البطن", "Abs", "متوسط", "Intermediate", "3 مجموعات × 12–15 تكرار", "3 sets × 12–15 reps", "ارفع ركبتيك نحو صدرك بتحكم، وتجنب التأرجح.", "Raise your knees toward your chest with control and avoid swinging.", "knee_raises.jpg"},
	{"deadlift", "الرفعة الميتة", "Deadlift", "back", "الظهر", "Back", "متقدم", "Advanced", "3 مجموعات × 8–12 تكرار", "3 sets × 8–12 reps", "تعلم حركة مفصل الحوض وابدأ بوزن خفيف مع ظهر محايد، ويفضل بإشراف مدرب.", "Learn the hip hinge and start light with a neutral back, ideally with a coach's guidance.", "pullups.jpg"},
	{"benchpress", "ضغط البنش", "Bench Press", "chest", "الصدر", "Chest", "متوسط", "Intermediate", "3 مجموعات × 8–12 تكرار", "3 sets × 8–12 reps", "ثبت القدمين والكتفين، واستخدم مساعدًا عند رفع الأوزان الثقيلة.", "Keep your feet and shoulders stable; use a spotter for heavy lifts.", "pushups.jpg"},
	{"legpress", "ضغط الأرجل بالجهاز", "Leg Press", "legs", "الأرجل", "Legs", "متوسط", "Intermediate", "3 مجموعات × 10–15 تكرار", "3 sets × 10–15 reps", "لا تقفل الركبتين تمامًا، واستخدم مدى حركة مريحًا.", "Avoid locking your knees and use a comfortable range of motion.", "squats.jpg"},
	{"triceps", "تمرين الترايسبس", "Triceps Pushdown", "arms", "الذراعان", "Arms", "مبتدئ", "Beginner", "3 مجموعات × 10–12 تكرار", "3 sets × 10–12 reps", "ثبت المرفقين بجانب الجسم وادفع المقبض لأسفل بتحكم.", "Keep your elbows by your sides and press the handle down with control.", "biceps_curl.jpg"},
	{"calfraises", "رفع السمانة", "Calf Raises", "legs", "الأرجل", "Legs", "مبتدئ", "Beginner", "3 مجموعات × 15 تكرار", "3 sets × 15 reps", "ارفع الكعبين ببطء ثم انزل بتحكم مع الحفاظ على التوازن.", "Raise your heels slowly, then lower with control while balancing.", "lunges.jpg"},
	{"lateralraises", "الرفرفة الجانبية", "Lateral Raises", "shoulders", "الأكتاف", "Shoulders", "مبتدئ", "Beginner", "3 مجموعات × 12 تكرار", "3 sets × 12 reps", "ارفع الأوزان للجانبين حتى مستوى الكتف تقريبًا دون هز الجسم.", "Lift the weights out to the sides to about shoulder height without swinging.", "shoulder_press.jpg"},
	{"mountainclimbers", "متسلق الجبل", "Mountain Climbers", "abs", "البطن", "Abs", "متوسط", "Intermediate", "3 مجموعات × 30 ثانية", "3 sets × 30 sec", "ابدأ بوضع الضغط وبدّل الركبتين نحو الصدر بإيقاع مناسب.", "Start in a push-up position and alternate driving your knees toward your chest.", "plank.jpg"},
	{"seatedrow", "السحب الأرضي", "Seated Row", "back", "الظهر", "Back", "متوسط", "Intermediate", "3 مجموعات × 10–12 تكرار", "3 sets × 10–12 reps", "اسحب المقبض باتجاه الجذع مع إبقاء الظهر ثابتًا وعدم رفع الكتفين.", "Pull the handle toward your torso while keeping your back steady and shoulders relaxed.", "pullups.jpg"},
}

var tmpl = template.Must(template.New("page").Parse(`<!doctype html>
<html lang="{{if .IsEnglish}}en{{else}}ar{{end}}" dir="{{if .IsEnglish}}ltr{{else}}rtl{{end}}">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}</title>
<style>
:root{--bg:#07131d;--panel:#102332;--panel2:#132b3b;--line:#244252;--text:#f3f8fb;--muted:#a8bdc9;--green:#28e0a0;--lime:#b8f36b}
*{box-sizing:border-box}body{margin:0;font-family:Inter,"Segoe UI",Tahoma,Arial,sans-serif;background:var(--bg);color:var(--text);line-height:1.65}
a{color:inherit;text-decoration:none}.wrap{max-width:1240px;margin:auto;padding:0 24px}
header{border-bottom:1px solid var(--line);background:#091722;position:sticky;top:0;z-index:5}.nav{display:flex;align-items:center;justify-content:space-between;gap:18px;min-height:72px}.brand{font-size:1.5rem;font-weight:900;letter-spacing:.5px}.brand span{color:var(--green)}.brand small{display:block;color:var(--muted);font-size:.7rem;font-weight:500;letter-spacing:0}.navlinks{display:flex;gap:22px;align-items:center;flex-wrap:wrap}.navlinks a{font-size:.92rem;color:var(--muted)}.navlinks a:hover,.lang{color:var(--green)!important}.lang{border:1px solid var(--line);border-radius:999px;padding:7px 12px}
.hero{padding:42px 0 32px;display:grid;grid-template-columns:1.4fr .8fr;gap:24px;align-items:center}.eyebrow{color:var(--green);font-weight:800;font-size:.9rem}.hero h1{font-size:clamp(2.2rem,5vw,4rem);line-height:1.15;margin:12px 0}.hero p{color:var(--muted);max-width:650px}.hero-box{border:1px solid var(--line);border-radius:22px;padding:22px;background:linear-gradient(135deg,#122a39,#0b1c28)}.benefit{display:flex;align-items:center;gap:12px;padding:10px 0;border-bottom:1px solid var(--line)}.benefit:last-child{border:0}.icon{display:grid;place-items:center;width:42px;height:42px;border-radius:50%;background:#143d39;color:var(--green);font-size:1.2rem;flex-shrink:0}.button{display:inline-block;background:var(--green);color:#042017;padding:11px 18px;border-radius:11px;font-weight:800;border:0;cursor:pointer;font:inherit}.button:hover{filter:brightness(1.08)}
.section{padding:24px 0}.heading{display:flex;justify-content:space-between;align-items:end;gap:15px;flex-wrap:wrap}.heading h2{margin:0;font-size:1.7rem}.muted{color:var(--muted)}.categories{display:flex;gap:10px;flex-wrap:wrap;margin:18px 0}.categories a{border:1px solid var(--line);border-radius:12px;background:var(--panel);padding:10px 16px;color:var(--muted)}.categories a.active,.categories a:hover{border-color:var(--green);color:var(--green);background:#10332e}
.controls{display:flex;gap:10px;flex-wrap:wrap;margin:18px 0}.controls input,.controls select{background:#0c1d29;border:1px solid var(--line);border-radius:10px;color:var(--text);padding:12px;font:inherit}.controls input{flex:1;min-width:220px}.grid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:16px}.card{overflow:hidden;border:1px solid var(--line);border-radius:16px;background:var(--panel);transition:transform .18s,border-color .18s}.card:hover{transform:translateY(-3px);border-color:var(--green)}.card img{display:block;width:100%;height:155px;object-fit:cover;background:#243643}.body{padding:15px}.card h3{font-size:1.05rem;margin:0 0 4px}.sub{font-size:.88rem;color:var(--muted)}.pill{display:inline-block;background:#123c32;color:var(--green);border-radius:999px;padding:3px 9px;font-size:.78rem;margin:8px 0}.sets{font-weight:700;font-size:.9rem}.desc{font-size:.88rem;color:var(--muted);min-height:70px}.details{border-top:1px solid var(--line);padding-top:10px;color:var(--green);font-size:.85rem}
.tips{display:grid;grid-template-columns:repeat(3,1fr);gap:14px}.tip{border:1px solid var(--line);border-radius:15px;padding:18px;background:var(--panel)}.tip strong{color:var(--green)}.empty{padding:28px;border:1px dashed var(--line);border-radius:14px;color:var(--muted)}footer{border-top:1px solid var(--line);margin-top:34px;padding:24px 0;color:var(--muted);font-size:.9rem}
@media(max-width:980px){.grid{grid-template-columns:repeat(3,minmax(0,1fr))}.hero{grid-template-columns:1fr}.navlinks{gap:12px}}@media(max-width:680px){.wrap{padding:0 15px}.grid{grid-template-columns:repeat(2,minmax(0,1fr))}.nav{padding:12px 0;align-items:flex-start}.navlinks{justify-content:flex-end}.hero{padding-top:28px}.tips{grid-template-columns:1fr}}@media(max-width:430px){.grid{grid-template-columns:1fr}.brand{font-size:1.2rem}.navlinks a:not(.lang){display:none}}
</style></head><body>
<header><div class="wrap nav"><a class="brand" href="/?lang={{.Lang}}">FIT<span>ZONE</span><small>{{if .IsEnglish}}Stronger Every Day{{else}}أقوى كل يوم{{end}}</small></a>
<nav class="navlinks"><a href="/?lang={{.Lang}}">{{if .IsEnglish}}Home{{else}}الرئيسية{{end}}</a><a href="#exercises">{{if .IsEnglish}}Exercises{{else}}التمارين{{end}}</a><a href="#tips">{{if .IsEnglish}}Tips{{else}}نصائح{{end}}</a><a class="lang" href="/?lang={{if .IsEnglish}}ar{{else}}en{{end}}">{{if .IsEnglish}}العربية{{else}}English{{end}} 🌐</a></nav></div></header>
<main class="wrap">
<section class="hero"><div><div class="eyebrow">{{if .IsEnglish}}FITNESS • STRENGTH • HEALTH{{else}}لياقة • قوة • صحة{{end}}</div><h1>{{if .IsEnglish}}A better life<br><span style="color:var(--green)">starts here.</span>{{else}}حياة أفضل<br><span style="color:var(--green)">تبدأ من هنا.</span>{{end}}</h1><p>{{if .IsEnglish}}Explore gym basics and exercise instructions, find movements by muscle group, and build healthy habits one workout at a time.{{else}}اكتشف أساسيات الجيم وطريقة أداء التمارين، وابحث حسب العضلة المستهدفة، وابنِ عادات صحية تمرينًا بعد تمرين.{{end}}</p><a class="button" href="#exercises">{{if .IsEnglish}}Explore exercises ↓{{else}}استعرض التمارين ↓{{end}}</a></div>
<div class="hero-box"><h2>{{if .IsEnglish}}Your fitness essentials{{else}}أساسيات رحلتك الرياضية{{end}}</h2><div class="benefit"><span class="icon">♡</span><div><strong>{{if .IsEnglish}}Better health{{else}}صحة أفضل{{end}}</strong><div class="muted">{{if .IsEnglish}}Move regularly{{else}}حافظ على نشاطك بانتظام{{end}}</div></div></div><div class="benefit"><span class="icon">↗</span><div><strong>{{if .IsEnglish}}Build strength{{else}}زيادة القوة{{end}}</strong><div class="muted">{{if .IsEnglish}}Progress gradually{{else}}تدرج في مستوى التمرين{{end}}</div></div></div><div class="benefit"><span class="icon">✓</span><div><strong>{{if .IsEnglish}}Stay consistent{{else}}استمر بانتظام{{end}}</strong><div class="muted">{{if .IsEnglish}}Rest and recover{{else}}اهتم بالراحة والتعافي{{end}}</div></div></div></div></section>
<section class="section"><div class="heading"><div><h2>{{if .IsEnglish}}Exercise categories{{else}}أقسام التمارين{{end}}</h2><div class="muted">{{if .IsEnglish}}Choose a target muscle group{{else}}اختار العضلة المستهدفة{{end}}</div></div></div><div class="categories"><a class="{{if eq .Category ""}}active{{end}}" href="/?lang={{.Lang}}">{{if .IsEnglish}}All{{else}}الكل{{end}}</a>{{range .Categories}}<a class="{{if eq $.Category .Key}}active{{end}}" href="/?lang={{$.Lang}}&category={{.Key}}">{{.Label}}</a>{{end}}</div></section>
<section class="section" id="exercises"><div class="heading"><div><h2>{{if .IsEnglish}}Exercise library{{else}}مكتبة التمارين{{end}}</h2><div class="muted">{{if .IsEnglish}}{{len .Exercises}} exercises available{{else}}عدد التمارين المعروضة: {{len .Exercises}}{{end}}</div></div></div>
<form class="controls" method="get" action="/"><input type="hidden" name="lang" value="{{.Lang}}"><input type="hidden" name="category" value="{{.Category}}"><input name="q" value="{{.Search}}" placeholder="{{if .IsEnglish}}Search exercises...{{else}}ابحث عن تمرين...{{end}}"><button class="button" type="submit">{{if .IsEnglish}}Search{{else}}بحث{{end}}</button></form>
{{if .Exercises}}<div class="grid">{{range .Exercises}}<article class="card"><img src="/static/images/{{.Image}}" alt="{{.Name}}" loading="lazy"><div class="body"><h3>{{.Name}}</h3><div class="sub">{{.Category}} · {{.Level}}</div><span class="pill">{{.Level}}</span><div class="sets">{{.Sets}}</div><p class="desc">{{.Description}}</p><div class="details">↗ {{if $.IsEnglish}}Exercise guide{{else}}دليل التمرين{{end}}</div></div></article>{{end}}</div>{{else}}<div class="empty">{{if .IsEnglish}}No matching exercises. Try another search.{{else}}لا توجد تمارين مطابقة. جرّب البحث بكلمة أخرى.{{end}}</div>{{end}}</section>
<section class="section" id="tips"><h2>{{if .IsEnglish}}Beginner tips{{else}}نصائح للمبتدئين{{end}}</h2><div class="tips" style="margin-top:16px"><div class="tip"><strong>{{if .IsEnglish}}01 · Warm up{{else}}١ · ابدأ بالإحماء{{end}}</strong><p class="muted">{{if .IsEnglish}}Warm up for 5–10 minutes before training.{{else}}خصص من 5 إلى 10 دقائق للإحماء قبل التمرين.{{end}}</p></div><div class="tip"><strong>{{if .IsEnglish}}02 · Good technique{{else}}٢ · أداء صحيح{{end}}</strong><p class="muted">{{if .IsEnglish}}Learn the movement before increasing weight.{{else}}تعلم الحركة الصحيحة قبل زيادة الأوزان.{{end}}</p></div><div class="tip"><strong>{{if .IsEnglish}}03 · Recovery matters{{else}}٣ · اهتم بالراحة{{end}}</strong><p class="muted">{{if .IsEnglish}}Sleep well, drink water, and stop if you feel sharp pain.{{else}}نم جيدًا واشرب الماء وتوقف إذا شعرت بألم حاد.{{end}}</p></div></div></section>
<footer>FITZONE · {{if .IsEnglish}}FITZONEا{{end}}</footer>
</main></body></html>`))

func main() {
	mongoURI := os.Getenv("MONGODB_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://127.0.0.1:27017"
	}
	dbName := os.Getenv("MONGODB_DATABASE")
	if dbName == "" {
		dbName = "fitzone"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		cancel()
		log.Fatalf("connect to MongoDB: %v", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		cancel()
		log.Fatalf("MongoDB is not reachable at %s: %v", mongoURI, err)
	}
	cancel()
	defer func() {
		if err := client.Disconnect(context.Background()); err != nil {
			log.Printf("disconnect MongoDB: %v", err)
		}
	}()

	database := client.Database(dbName)
	collection := database.Collection("exercises")
	seedCtx, seedCancel := context.WithTimeout(context.Background(), 15*time.Second)
	count, err := collection.CountDocuments(seedCtx, bson.D{})
	if err != nil {
		seedCancel()
		log.Fatalf("count exercises: %v", err)
	}
	if count == 0 {
		docs := make([]interface{}, 0, len(allExercises))
		for _, exercise := range allExercises {
			docs = append(docs, exercise)
		}
		if len(docs) > 0 {
			if _, err := collection.InsertMany(seedCtx, docs); err != nil {
				seedCancel()
				log.Fatalf("seed exercises: %v", err)
			}
		}
		log.Printf("Seeded %d exercises into MongoDB", len(docs))
	}
	seedCancel()

	dbExercises := func() []exerciseText {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		cursor, err := collection.Find(ctx, bson.D{})
		if err != nil {
			log.Printf("load exercises from MongoDB: %v", err)
			return nil
		}
		defer cursor.Close(ctx)
		var result []exerciseText
		if err := cursor.All(ctx, &result); err != nil {
			log.Printf("decode exercises from MongoDB: %v", err)
			return nil
		}
		return result
	}

	registerAPIRoutes(database)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		lang := strings.ToLower(r.URL.Query().Get("lang"))
		isEN := lang == "en"
		if !isEN {
			lang = "ar"
		}
		category := strings.TrimSpace(r.URL.Query().Get("category"))
		search := strings.TrimSpace(r.URL.Query().Get("q"))
		cats := []Category{
			{"chest", choose(isEN, "Chest", "الصدر")}, {"back", choose(isEN, "Back", "الظهر")},
			{"legs", choose(isEN, "Legs", "الأرجل")}, {"shoulders", choose(isEN, "Shoulders", "الأكتاف")},
			{"arms", choose(isEN, "Arms", "الذراعان")}, {"abs", choose(isEN, "Abs", "البطن")},
		}
		list := make([]Exercise, 0, len(allExercises))
		for _, e := range dbExercises() {
			name, cat, level, sets, desc := e.NameAR, e.CategoryAR, e.LevelAR, e.SetsAR, e.DescAR
			if isEN {
				name, cat, level, sets, desc = e.NameEN, e.CategoryEN, e.LevelEN, e.SetsEN, e.DescEN
			}
			if category != "" && category != e.CategoryKey {
				continue
			}
			haystack := strings.ToLower(name + " " + e.NameAR + " " + e.NameEN + " " + cat + " " + e.CategoryAR + " " + e.CategoryEN + " " + desc)
			if search != "" && !strings.Contains(haystack, strings.ToLower(search)) {
				continue
			}
			list = append(list, Exercise{Name: name, Category: cat, Level: level, Sets: sets, Description: desc, Image: e.Image, Key: e.Key})
		}
		data := PageData{Title: "FITZONE | Gym & Exercises", Lang: lang, Search: search, Category: category, IsEnglish: isEN, Exercises: list, Categories: cats}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.Execute(w, data); err != nil {
			log.Printf("render page: %v", err)
		}
	})
	log.Println("FITZONE is running at http://localhost:4000 (MongoDB connected; REST API enabled)")
	log.Fatal(http.ListenAndServe(":4000", nil))
}
func choose(en bool, english, arabic string) string {
	if en {
		return english
	}
	return arabic
}
