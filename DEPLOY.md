# دليل نشر منهل

البوت يعمل بـ long-polling (لا يحتاج رابطاً عاماً). تحتاج فقط: Docker + Docker
Compose، و(اختياري) نطاق + HTTPS للوحة الأدمن.

## 1) الأسرار (لا تضعها في git — في `.env` أو خزنة Coolify فقط)
| المتغيّر | إلزامي | المصدر |
|---|---|---|
| `BOT_TOKEN` | نعم | @BotFather |
| `POSTGRES_PASSWORD` | نعم | تختاره |
| `ADMIN_IDS` | نعم (لـ `/admin` بالبوت) | @userinfobot |
| `ADMIN_WEB_TOKEN` | نعم (للوحة الويب) | تختاره |
| `DEEPSEEK_API_KEY` | للذكاء | platform.deepseek.com |
| `EMBED_MODEL` | للبحث الدلالي/PDF | `nomic-embed-text` خفيف (يُنزَّل تلقائياً)؛ يمكن الترقية لـ `bge-m3` على خادم قوي |
| `UNPAYWALL_EMAIL` | لميزة OA | بريدك |

> `DATABASE_URL` و`OLLAMA_URL` يُضبَطان تلقائياً في `docker-compose.yml` — لا تضعهما.

## 2) النشر عبر Docker Compose (VPS)
```bash
git clone <repo> && cd manhal
cp .env.example .env && nano .env     # املأ الأسرار أعلاه
docker compose up -d --build
docker compose logs -f bot            # تابع: "منهل bot starting"
```
خدمة `ollama-pull` تُنزّل `nomic-embed-text` تلقائياً (دقائق)؛ يصبح البحث الدلالي ومحادثة
PDF جاهزين بعد اكتمالها.

## 3) النشر عبر Coolify (موصى به لـ HTTPS تلقائي)
1. `+ New Resource → Docker Compose` واربطه بالمستودع.
2. أضِف الأسرار في تبويب **Environment Variables**.
3. اربط نطاقاً فرعياً (مثل `manhal-admin.example.com`) بخدمة `bot` منفذ `8080` —
   Coolify يصدر شهادة HTTPS تلقائياً عبر Traefik.
4. **Deploy**، ثم نزّل النموذج إن لم يكتمل: طرفية حاوية `ollama` → `ollama pull nomic-embed-text`.

## 4) الأمان
- لا تعرّض `:8080` على الإنترنت بلا HTTPS — استخدم نطاق Coolify أو Cloudflare Tunnel.
- نسخة واحدة فقط بنفس `BOT_TOKEN` (تشغيل نسختين = تعارض polling).
- دوّر أي سرّ يُكشَف فوراً.

## 5) ثبات البيانات والنسخ الاحتياطي (مهم)
**كل بيانات التشغيل (المستخدمون، المكتبة، طلبات الدعم، الاشتراكات، مراقبة الاستشهاد)
في Postgres، على volume منفصل `pgdata`.** عند تحديث الكود يُعاد بناء حاوية `bot`
فقط — أمّا `pgdata` فيبقى سليماً. **لا تُمسح البيانات إلا بأمر صريح**
`docker compose down -v` (لا تستخدمه إلا متعمّداً).

نسخ احتياطي تلقائي: خدمة `db-backup` تأخذ نسخة يوميّة إلى volume منفصل `pgbackups`
وتحتفظ بآخر 14 نسخة.

**استعادة نسخة** عند الحاجة:
```bash
# اعرض النسخ المتاحة
docker compose run --rm -v manhal_pgbackups:/backups db-backup ls -1 /backups
# استعِد نسخة محدّدة
docker compose exec -T postgres pg_restore -U manhal -d manhal --clean --if-exists \
  < $(docker volume inspect manhal_pgbackups -f '{{.Mountpoint}}')/manhal-<TIMESTAMP>.dump
```

> في Coolify: الـ named volumes (`pgdata`, `pgbackups`, `ollama`) تبقى محفوظة عبر
> عمليات إعادة النشر — Coolify لا يحذفها تلقائياً، ويُحذّرك قبل أي إزالة.

### ثبات تعديلات القائمة (اختياري)
تعديلات `/admin` تُكتب في `data/menu.yaml` داخل الحاوية. لإبقائها بعد إعادة البناء
أضِف volume لخدمة `bot` (`- menudata:/app/data`). ملاحظة: هذا يُخفي بيانات البذرة عند
أول تشغيل؛ الأنظف لاحقاً نقل القائمة إلى قاعدة البيانات.
