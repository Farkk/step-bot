# ШАГ — кабинет заказчика, бот и Mini App

Реализация следует [технической спецификации](TECH_SPEC.md): один Go-процесс обслуживает webhook Max, HTTP и собранный React Mini App. PostgreSQL и MinIO работают в отдельных контейнерах. Веб-кабинет заказчика и Mini App работают через общий Go API.

Для дальнейшей разработки используйте [skill проекта](skills/step-project/SKILL.md) и [живую документацию реализации](docs/PROJECT_GUIDE.md). Визуальные правила находятся в [дизайн-системе](DESIGN_SYSTEM.md).

## Запуск в OrbStack

Нужен запущенный OrbStack с Docker. Из корня проекта:

```sh
cp .env.example .env
docker compose up --build -d
docker compose ps
```

Mini App: <http://localhost:8080/app/>. Проверка готовности: <http://localhost:8080/health/ready>. В OrbStack должны отображаться контейнеры `step-app-1`, `step-postgres-1`, `step-minio-1`.

Если порт 8080 занят, задайте `APP_HOST_PORT` в `.env`, например `APP_HOST_PORT=8081`. Остановка: `docker compose down`. Именованные тома при обычной остановке сохраняются.

При запуске контейнер приложения выполняет версионированные SQL-миграции командой `server migrate`, затем запускает HTTP-сервер. Миграцию можно запустить отдельно: `docker compose run --rm app /app/server migrate`.

## Развёртывание на VPS

На VPS с Docker Compose и общей сетью Traefik `proxy` клонируйте репозиторий из GitHub. Создайте локальный `.env` на сервере по образцу `.env.example`: задайте `APP_ENV=production`, `PUBLIC_BASE_URL=https://step-bot.madebypavel.space`, токен MAX и уникальные `POSTGRES_PASSWORD`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`, `MAX_WEBHOOK_SECRET`, `SESSION_SECRET`. Не добавляйте `.env` в Git.

```sh
git clone https://github.com/Farkk/step-bot.git /opt/apps/step-bot.madebypavel.space
cd /opt/apps/step-bot.madebypavel.space
docker compose -f compose.yaml -f compose.vps.yaml up --build -d
docker compose -f compose.yaml -f compose.vps.yaml ps
```

`compose.vps.yaml` подключает приложение к сети `proxy`, закрывает прямой доступ к его порту и включает HTTPS через имеющийся Traefik с Let's Encrypt. PostgreSQL и MinIO остаются только во внутренней сети проекта. После запуска проверьте `/health/ready`, `/app/` и `/admin/`. Для обновления выполните `git pull --ff-only` и повторите `docker compose -f compose.yaml -f compose.vps.yaml up --build -d`. Перед обновлением сделайте резервную копию PostgreSQL и тома MinIO.

После получения сертификата настройте в MAX Mini App URL `https://step-bot.madebypavel.space/app/` и подписку на `https://step-bot.madebypavel.space/integrations/max/webhook` с тем же `MAX_WEBHOOK_SECRET`, который указан в `.env`.

## Структура

- `cmd/server` — точка входа, миграции и корректная остановка сервера.
- `internal/config` — конфигурация окружения.
- `internal/httpserver` — HTTP-маршруты, проверки здоровья и SPA fallback.
- `internal/max` — приём событий Max, исходящие сообщения и очередь push.
- `internal/tasks` — заявки, отклики, callback Max, оценка и аналитика.
- `internal/storage` и `migrations` — PostgreSQL, S3 и версионированная схема.
- `web/miniapp/src/app` — композиция Mini App; `src/shared/api` — общий HTTP-клиент. Новые сценарии удобно размещать в `src/features` по функциям.
- `infra/minio` — инфраструктурный образ из [официального релиза MinIO](https://github.com/minio/minio/releases/tag/RELEASE.2025-09-07T16-13-09Z) с проверкой контрольной суммы.

## Что работает сейчас

`POST /integrations/max/webhook` проверяет `X-Max-Bot-Api-Secret`, ограничивает тело 1 МиБ и сохраняет JSON в `max_webhook_inbox` до ответа `200`. Повтор того же тела не создаёт вторую запись. Фоновый обработчик принимает callback «Откликнуться» и команду `/tasks`; отправка карточек и push идёт через Max Bot API. `GET/PUT /api/v1/me/profile` проверяют данные запуска Max и позволяют сохранить профиль.

В `.env.example` указаны только локальные значения. При `APP_ENV=local` откройте [Mini App](http://localhost:8080/app/) в обычном браузере на `localhost`: форма работает без MAX, а данные сохраняются в локальную PostgreSQL. После сохранения можно нажать «Изменить данные», чтобы снова посмотреть форму. Локальный режим не работает на внешнем адресе и при другом `APP_ENV`. Для проверки в настоящем MAX задайте `MAX_BOT_TOKEN` от связанного бота. Перед размещением на VPS задайте свои секреты, HTTPS-домен и настройте подписку webhook.

[Кабинет заказчика](http://localhost:8080/admin/) использует вход по email и паролю. Первый аккаунт владельца создаётся разово после запуска и миграций:

```sh
read -s ADMIN_INITIAL_PASSWORD
export ADMIN_INITIAL_PASSWORD
docker compose exec -e ADMIN_INITIAL_PASSWORD app /app/server create-owner "Название компании" owner@example.ru "Имя владельца"
unset ADMIN_INITIAL_PASSWORD
```

Пароль должен содержать не менее 12 символов. Команда создаёт новую компанию и её владельца; публичной регистрации нет. Владелец может добавить менеджеров и наблюдателей в разделе «Команда», настроить дополнительные поля по типу заявки. Каждый сотрудник может сменить свой пароль. В кабинете доступны заявки с бюджетом, сроком, координатами и вложениями (изображения, PDF, TXT, ZIP до 5 МБ на файл), решения по откликам, журнал событий, оценка с обязательным комментарием в течение 14 дней после закрытия, аналитика и CSV. Файлы хранятся в S3/MinIO. В Mini App исполнители видят открытые заявки, откликаются, ведут заказы и видят свою репутацию. При заданном `MAX_BOT_TOKEN` сервер доставляет карточки и изменения статуса через Bot API Max; без токена сообщения остаются в очереди. В кабинете заказчик может приостановить, возобновить, отменить заявку и подтвердить завершение после действия исполнителя.

Локальные проверки:

```sh
go test ./...
cd web/miniapp && npm ci && npm test && npm run build
cd ../admin && npm ci && npm run build
```
