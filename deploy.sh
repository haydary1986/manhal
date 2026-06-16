#!/usr/bin/env bash
# نشر منهل على خادمك. شغّله أنت على الخادم بعد تدوير أسرارك.
# الاستخدام:  ./deploy.sh
set -euo pipefail

cd "$(dirname "$0")"

if ! command -v docker >/dev/null 2>&1; then
  echo "❌ Docker غير مثبّت. ثبّته أولاً: https://docs.docker.com/engine/install/"
  exit 1
fi

if [ ! -f .env ]; then
  cp .env.example .env
  echo "📝 أُنشئ .env من القالب. عدّل الأسرار الآن ثم أعد التشغيل:"
  echo "   nano .env   # BOT_TOKEN, DEEPSEEK_API_KEY, ADMIN_IDS, ADMIN_WEB_TOKEN, POSTGRES_PASSWORD, UNPAYWALL_EMAIL"
  exit 0
fi

# تحقّق من المتغيّرات الإلزامية.
missing=()
for v in BOT_TOKEN POSTGRES_PASSWORD ADMIN_WEB_TOKEN; do
  grep -qE "^${v}=.+" .env || missing+=("$v")
done
if [ "${#missing[@]}" -gt 0 ]; then
  echo "❌ متغيّرات إلزامية ناقصة في .env: ${missing[*]}"
  exit 1
fi

echo "🚀 بناء وتشغيل الخدمات..."
docker compose up -d --build

echo "⏳ بانتظار جاهزية البوت..."
sleep 5
docker compose logs --tail=20 bot || true

echo
echo "✅ تم. تابع السجلّات:  docker compose logs -f bot"
echo "   لوحة الأدمن:        http://<host>:8080/admin  (المستخدم admin)"
echo "   نموذج التضمين bge-m3 يُنزَّل تلقائياً (دقائق) لتفعيل البحث الدلالي/PDF."
