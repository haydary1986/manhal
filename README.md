# منهل (Manhal) 🎓

بوت تلكرام أكاديمي موحّد للباحثين والطلبة في العراق — أدوات بحث واستشهاد وذكاء اصطناعي، مع قناة إعلانات مؤتمرات/منح.

> المواصفات الكاملة: [FEATURES.md](FEATURES.md) · متابعة التقدّم: [PROGRESS.md](PROGRESS.md)

## المعمارية
هجين: **نواة Go واحدة** تخدم **بوت تلكرام** (أولاً) و**منصة ويب** (لاحقاً، للأدمن والميزات الثقيلة).

```
cmd/server         نقطة الدخول (بوت + خادم أدمن ويب)
internal/config    تحميل env + إعدادات YAML + التخصصات
internal/domain    الأنواع الأساسية (User, Tier)
internal/store     واجهة التخزين (memory للتطوير + Postgres عبر pgx/v5)
internal/bot       adapter تلكرام (gate, menu, handlers, admin)
internal/web       adapter الويب (لوحة أدمن لإدارة الأزرار)
internal/menu      شجرة القائمة الهرمية القابلة للتعديل
internal/alerts    محرّك التنبيهات (مجدول يدفع الأوراق الجديدة)
internal/ai        مزوّد LLM مجرّد (DeepSeek)
internal/embed     طبقة embeddings مجرّدة (Ollama محلي) للبحث الدلالي
internal/pdf       استخراج نص PDF (نقي Go) لمحادثة الـ RAG
internal/assist    أدوات المساعد الذكي (قوالب prompt)
internal/scholar   Crossref + OpenAlex + Unpaywall (اقتباس/بحث/باحث/OA)
internal/cite      مولّد الاقتباسات (٦ صيغ + BibTeX)
internal/announce  محرّك الإعلانات + الفلترة بالتخصص
internal/journal   فاحص تصنيف المجلات (Scimago)
internal/predator  كاشف المجلات المفترسة (استرشادي)
internal/promotion حاسبة الترقيات (تعليمات ١٠/٢٠٢٥)
internal/stats     المساعد الإحصائي (T/ANOVA/بيرسون/كرونباخ)
internal/latex     حماية المعادلات/الأكواد أثناء التدقيق (P11)
internal/docx      استخراج نص Word للتدقيق (P11)
internal/billing   طبقة الدفع (نائمة حتى تفعيل التحصيل)
data/              ملفات إعدادات قابلة للتعديل (YAML/CSV)
```

## التشغيل محلياً
```bash
cp .env.example .env   # ثم ضع BOT_TOKEN
go run ./cmd/server
```
ثم في تلكرام: افتح البوت واكتب `/start`.

## المتغيّرات
| المتغيّر | الوصف |
|---|---|
| `BOT_TOKEN` | توكن BotFather (مطلوب) |
| `DATABASE_URL` | رابط Postgres (فارغ = تخزين بالذاكرة للتطوير) |
| `DEEPSEEK_API_KEY` | مفتاح DeepSeek (فارغ = AI معطّل) |
| `CROSSREF_MAILTO` | بريد مجمّع Crossref/OpenAlex المهذّب (اختياري) |
| `UNPAYWALL_EMAIL` | بريد Unpaywall (يرجع لـ `CROSSREF_MAILTO`) |
| `ADMIN_IDS` | معرّفات أدمن البوت مفصولة بفواصل |
| `WEB_ADDR` | عنوان خادم لوحة الويب (افتراضي `:8080`) |
| `ADMIN_WEB_TOKEN` | كلمة مرور الأدمن الرئيسي (اسم المستخدم `admin`) |
| `ADMIN_WEB_USERS` | أدمنون إضافيون `user:pass` مفصولون بفواصل |

## لوحة الأدمن (الويب)
فعّل `ADMIN_WEB_TOKEN` و/أو `ADMIN_WEB_USERS` (يدعم **أكثر من أدمن**) ثم افتح `http://<host>:8080/admin` (Basic Auth):
- **إدارة الأزرار** (`/admin`): أزرار البوت **هرمية وقابلة للتعديل** دون إعادة نشر؛ تشترك مع أمر `/admin` بالبوت في نفس البيانات (`data/menu.yaml`) وتظهر فوراً.
- **الدعم الفني** (`/admin/support`): مراجعة طلبات المستخدمين والرد عليها؛ يصل الرد للمستخدم تلقائياً عبر البوت.

اللوحة لا تعمل إطلاقاً بدون حساب واحد على الأقل، وتبقى متاحة حتى لو تعذّر اتصال البوت بتلكرام.

## الأمان
لا تضع الأسرار في الكود. `.env` مستثنى من git. لوحة الويب لا تعمل إطلاقاً بدون `ADMIN_WEB_TOKEN`. مفاتيح النشر (Coolify/Cloudflare) تبقى خارج المستودع.
