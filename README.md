# ШАГ — кабинет заказчика, бот и Mini App

Серверная версия находится в `backend/`: Laravel 13, MySQL, бот MAX с отправкой уведомлений сразу после ответа API и повторными попытками через cron. React Mini App и кабинет остались прежними. На действующем VPS и домене работает PHP-версия; прежний Go-контейнер остановлен. Текущий статус переноса описан в [руководстве проекта](docs/PROJECT_GUIDE.md).

Для дальнейшей разработки используйте [skill проекта](skills/step-project/SKILL.md) и [живую документацию реализации](docs/PROJECT_GUIDE.md). Визуальные правила находятся в [дизайн-системе](DESIGN_SYSTEM.md).

## Локальный запуск Laravel + MySQL

Нужны Docker/OrbStack и Node.js для сборки двух существующих React-клиентов. Из корня проекта:

```sh
cp backend/.env.example backend/.env
cd backend && composer install && php artisan key:generate && cd ..
sh scripts/build-php-assets.sh
docker compose -f compose.php.yaml up --build -d
docker compose -f compose.php.yaml exec -T app php artisan migrate --force
docker compose -f compose.php.yaml ps
```

В `compose.php.yaml` для локального просмотра включены `APP_ENV=local`, MySQL и порт `127.0.0.1:8082`. Mini App: <http://localhost:8082/app/>, кабинет: <http://localhost:8082/admin/>, готовность: <http://localhost:8082/health/ready>. Пробный проход очереди: `docker compose -f compose.php.yaml exec -T app php artisan max:tick`. Тесты: `cd backend && php artisan test --compact`. База новой версии независима от PostgreSQL прежнего сервера.

Первого владельца создайте после миграций. Задайте пароль длиной от 12 символов в `ADMIN_INITIAL_PASSWORD` только на время команды:

```sh
read -r -s ADMIN_INITIAL_PASSWORD
export ADMIN_INITIAL_PASSWORD
docker compose -f compose.php.yaml exec -T -e ADMIN_INITIAL_PASSWORD app php artisan owner:create 'Компания' owner@example.ru 'Имя владельца'
unset ADMIN_INITIAL_PASSWORD
```

## Развёртывание на обычном Beget

Для установки из Git с уже готовыми React-сборками используйте [короткую инструкцию Beget](docs/BEGET_FROM_GIT.md). Указанный там домен служит примером. Для Beget выбран новый бот `id615421905600_bot`: нужны его токен, новый webhook secret и отдельная подписка MAX. Подробности по настройке хостинга, подключению домена и проверкам — в [полной инструкции](docs/BEGET_SHARED_HOSTING.md).

Схема каталога `public_html` → `public` и выбор PHP 8.3 соответствуют [инструкции Beget для Laravel](https://beget.com/ru/kb/how-to/web-apps/ustanovka-php-frejmvorkov). Версию сайта и PHP-директивы можно настроить в [панели сайтов Beget](https://beget.com/ru/kb/manual/sajty).

Требуются PHP 8.3+ для сайта и CLI, MySQL, HTTPS, cron раз в минуту, расширения `mbstring`, `openssl`, `pdo_mysql`, `fileinfo`, `zip`. Тариф и фактическую версию PHP/MySQL следует проверить в панели Beget. Для вложений до 5 МБ выставьте `upload_max_filesize` не менее 6 МБ и `post_max_size` не менее 7 МБ в настройках PHP сайта. Сборки React уже лежат в `backend/public/app` и `backend/public/admin`; если исходники менялись, сначала выполните `sh scripts/build-php-assets.sh` локально.

1. Клонируйте основную ветку GitHub в каталог сайта, вне `public_html`. Для SSH-сессии Beget можно выбрать PHP 8.3 через `export PATH=/usr/local/php/cgi/8.3/bin/:$PATH`. В `project/backend` установите PHP-зависимости командой `composer install --no-dev --prefer-dist --optimize-autoloader`.
2. Создайте `backend/.env` по `backend/.env.example`. Укажите `APP_ENV=production`, `APP_DEBUG=false`, реальный HTTPS `APP_URL`, доступ к MySQL, `MAX_BOT_TOKEN`, публичное имя бота в `MAX_BOT_WEB_APP` (для кнопки Mini App) и случайный `MAX_WEBHOOK_SECRET`. Выполните `php artisan key:generate`, `php artisan migrate --force`, затем `php artisan owner:create ...` с временным `ADMIN_INITIAL_PASSWORD`.
3. Настройте `public_html` как ссылку на `project/backend/public` или укажите этот каталог корнем сайта в панели. **Веб-сервер не должен отдавать `.env`, `vendor`, `storage` и исходники PHP.** Дайте PHP право записи в `backend/storage` и `backend/bootstrap/cache`.
4. В Beget CronTab выберите тот же PHP 8.3+ CLI и поставьте запуск каждую минуту: `cd /абсолютный/путь/project/backend && /usr/local/php/cgi/8.3/bin/php artisan schedule:run >> storage/logs/cron.log 2>&1`. Уточните путь к PHP на своём сервере; cron повторяет неудачные отправки и подбирает оставшуюся очередь; обычные уведомления отправляются после ответа на действие без ожидания минуты.
5. Проверьте по HTTPS `/health/ready`, `/app/`, `/admin/`, вход владельца и тестовую заявку. После этого у нового бота задайте Mini App URL `https://<домен>/app/` и создайте его webhook-подписку на `https://<домен>/integrations/max/webhook` с тем же секретом, что в Beget `.env`.

Файлы заявок хранятся в приватном `backend/storage/app/private` и скачиваются через `/api/v1/attachments/{id}` с проверкой доступа. После успешных изменений через API и входящего webhook Laravel запускает обработку MAX после отправки HTTP-ответа. Большая очередь или сбой MAX могут задержать оставшиеся сообщения до cron. При обновлении кода повторите `composer install --no-dev`, `php artisan migrate --force` и очистите кэш командой `php artisan optimize:clear`; содержимое `storage` и `.env` сохраняйте.

Для очистки **всех** заявок запустите из `backend` команду `php artisan tasks:clear`: она только покажет число затронутых записей. После резервной копии MySQL и проверки списка выполните `php artisan tasks:clear --force`. Команда удаляет заявки, отклики, историю, оценки, уведомления, связанные фотоответы бота и файлы вложений. Профили исполнителей, компании и сотрудники сохраняются. Удаление необратимо без резервной копии.

## Действующая PHP-версия на VPS

Каталог `/opt/apps/step-bot-php` содержит копию `backend/`, `Dockerfile.php`, `.dockerignore`, `compose.php.vps.yaml` и `compose.php.vps.traefik.yaml`. Секреты находятся только в локальных серверных `.env`; код кабинета и Mini App совпадает с локальной сборкой. Для проверки:

```sh
ssh vps 'cd /opt/apps/step-bot-php && docker compose -f compose.php.vps.yaml -f compose.php.vps.traefik.yaml ps'
curl -fsS https://step-bot.madebypavel.space/health/ready
```

`app` обслуживает действующий домен через Traefik, `scheduler` каждую минуту запускает `php artisan schedule:run`, `mysql` хранит новую базу. При обновлении PHP-кода синхронизируйте `backend/` без `.env`, `vendor` и `storage`, затем выполните `docker compose -f compose.php.vps.yaml -f compose.php.vps.traefik.yaml up --build -d app scheduler` и `docker compose -f compose.php.vps.yaml exec -T app php artisan migrate --force`. Перед обновлением сохраняйте тома `step-php_mysql_data` и `step-php_php_storage`.

База заявок создана заново. Из прежней PostgreSQL перенесена только учетная запись владельца с действующим bcrypt-хешем, чтобы сохранить вход в кабинет. Go-приложение остановлено; его PostgreSQL и MinIO и тома сохранены для возможного отката. Подписка MAX уже направлена на `https://step-bot.madebypavel.space/integrations/max/webhook`; повторная регистрация не требуется, пока адрес и секрет остаются прежними.

## Прежняя Go-версия на VPS

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

Образ приложения добавляет [корневой](https://gu-st.ru/content/lending/russian_trusted_root_ca_pem.crt) и [промежуточный](https://gu-st.ru/content/lending/russian_trusted_sub_ca_pem.crt) сертификаты Минцифры из официального источника в хранилище доверенных CA: они нужны для HTTPS-запросов к `platform-api2.max.ru`.

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
