# Быстрый запуск «ШАГ» на обычном Beget из Git

В Git уже лежат Laravel, миграции, `composer.lock` и готовые сборки React для Mini App и кабинета. На Beget не нужны Node.js, Go, Docker и сборка фронтенда. Понадобятся PHP 8.3+ для сайта и SSH, MySQL 8, SSH, Composer 2, HTTPS и CronTab. В панели Beget создайте сайт для нужного домена и **пустую** базу MySQL. Ниже каталог сайта для примера — `~/step-bot.madebypavel.space`; если Beget создал другой, подставьте его.

## Первый запуск

Подключитесь к Beget по SSH. Клонируйте основную ветку **внутрь каталога сайта, но вне `public_html`**:

```sh
cd ~/step-bot.madebypavel.space
git clone --branch main https://github.com/Farkk/step-bot.git project
cd project/backend
export PATH=/usr/local/php/cgi/8.3/bin:$PATH
php -v
cp .env.example .env
chmod 600 .env
nano .env
```

В `.env` укажите `APP_ENV=production`, `APP_DEBUG=false`, `APP_URL=https://step-bot.madebypavel.space`, параметры новой MySQL (`DB_HOST=localhost`, `DB_DATABASE`, `DB_USERNAME`, `DB_PASSWORD`), а также `MAX_BOT_TOKEN`, `MAX_BOT_WEB_APP=se14459976_bot`, `MAX_WEBHOOK_SECRET`. Токен и секрет действующего бота возьмите из защищённого `backend/.env` на VPS (`/opt/apps/step-bot-php/backend/.env`); не публикуйте их и не копируйте весь файл, потому что там настройки другой базы. `APP_KEY` оставьте пустым до команды ниже.

Затем в том же каталоге:

```sh
mkdir -p storage/app/private storage/framework/cache/data storage/framework/sessions storage/framework/views storage/logs bootstrap/cache
chmod -R u+rwX storage bootstrap/cache
COMPOSER_MEMORY_LIMIT=-1 ~/.local/bin/composer install --no-dev --prefer-dist --optimize-autoloader --no-interaction
~/.local/bin/composer check-platform-reqs --no-dev
php artisan key:generate --force
php artisan migrate --force --no-interaction
php artisan max:tick
```

Если `~/.local/bin/composer` отсутствует, установите Composer 2 по [подробной инструкции](BEGET_SHARED_HOSTING.md#3-подготовить-php-зависимости-и-закрытые-каталоги). Если Composer сообщает об отсутствующем PHP-расширении или ошибке установки, сначала исправьте это на хостинге.

Создайте нового владельца кабинета; пароль от 12 символов вводится скрыто:

```sh
read -r -s ADMIN_INITIAL_PASSWORD
printf '\n'
export ADMIN_INITIAL_PASSWORD
php artisan owner:create 'ШАГ' '<ваша_почта>' '<ваше_имя>'
unset ADMIN_INITIAL_PASSWORD
```

Убедитесь, что `public_html` сейчас относится именно к этому сайту. Сохраните исходный каталог и откройте наружу только Laravel `public`:

```sh
cd ~/step-bot.madebypavel.space
mv public_html public_html.before-step
ln -s project/backend/public public_html
ls -l public_html/.htaccess public_html/app/index.html public_html/admin/index.html
```

В панели Beget выберите PHP 8.3+ и HTTPS для сайта. Для вложений выставьте `upload_max_filesize=6M` и `post_max_size=7M` или больше. В **CronTab** добавьте запуск каждую минуту (`* * * * *`); замените путь реальным результатом `pwd` в `project/backend`:

```sh
cd /home/<логин>/step-bot.madebypavel.space/project/backend && /usr/local/php/cgi/8.3/bin/php artisan schedule:run >> storage/logs/cron.log 2>&1
```

После перевода DNS домена на Beget проверьте по HTTPS `/health/ready`, `/app/`, `/admin/`, вход владельца и публикацию тестовой заявки. У действующего бота URL webhook остаётся `/integrations/max/webhook`; при прежнем `MAX_WEBHOOK_SECRET` повторная подписка не нужна. Новая база пуста, поэтому исполнитель должен заново сохранить профиль в Mini App, прежде чем получит пуш. Порядок переключения DNS, проверки MAX и сохранения VPS для возврата описан в [подробной инструкции](BEGET_SHARED_HOSTING.md#7-переключить-домен-с-vps-на-beget).

## Обновление из Git

Сохраняйте серверные `.env`, `APP_KEY`, MySQL и `storage/app/private`. Из SSH:

```sh
cd ~/step-bot.madebypavel.space/project
git pull --ff-only origin main
cd backend
export PATH=/usr/local/php/cgi/8.3/bin:$PATH
COMPOSER_MEMORY_LIMIT=-1 ~/.local/bin/composer install --no-dev --prefer-dist --optimize-autoloader --no-interaction
php artisan migrate --force --no-interaction
php artisan optimize:clear
```

Повторять `key:generate` и `owner:create` при обновлении не нужно. Полная диагностика и резервное копирование описаны в [подробной инструкции](BEGET_SHARED_HOSTING.md).
