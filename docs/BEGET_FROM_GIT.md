# Быстрый запуск «ШАГ» на обычном Beget из Git

В Git уже лежат Laravel, миграции, `composer.lock` и готовые сборки React для Mini App и кабинета. На Beget не нужны Node.js, Go, Docker и сборка фронтенда. Понадобятся PHP 8.3+ для сайта и SSH, MySQL 8, Composer 2, HTTPS и CronTab. В панели Beget создайте сайт для **своего будущего домена** и пустую базу MySQL. Ниже каталог сайта для примера — `~/step-bot.madebypavel.space`; если Beget создал другой, подставьте его во всех командах. Адрес домена можно выбрать позже, перед настройкой `.env` и MAX.

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

В `.env` укажите `APP_ENV=production`, `APP_DEBUG=false`, `APP_URL=https://<ваш-домен>` (без пути `/app/`), параметры новой MySQL (`DB_HOST=localhost`, `DB_DATABASE`, `DB_USERNAME`, `DB_PASSWORD`). Для **нового бота** `id615421905600_bot` укажите `MAX_BOT_TOKEN` из его настроек MAX, `MAX_BOT_WEB_APP=id615421905600_bot` и новый `MAX_WEBHOOK_SECRET`, который был создан для вас отдельно. То же значение секрета укажите при создании webhook-подписки; оно не выдаётся автоматически вместе с токеном. Если потеряете его, создайте другой командой `/usr/local/php/cgi/8.3/bin/php -r 'echo bin2hex(random_bytes(32)), PHP_EOL;'` и используйте новый в обоих местах. Не берите токен и секрет старого бота с VPS. `APP_KEY` оставьте пустым до команды ниже. Токен и секрет не добавляйте в Git.

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

Убедитесь, что `public_html` сейчас относится именно к этому сайту. **Сначала проверьте, где фактически лежит код:** файл `project/backend/public/index.php` должен существовать. Если проект клонирован в другое место, используйте его абсолютный путь вместо `"$(pwd)/project/backend/public"`. Для новой установки сохраните исходный каталог и откройте наружу только Laravel `public`:

```sh
cd ~/step-bot.madebypavel.space
test -f project/backend/public/index.php && mv public_html public_html.before-step && ln -s "$(pwd)/project/backend/public" public_html
readlink public_html
ls -l public_html/.htaccess public_html/app/index.html public_html/admin/index.html
```

Если `test` не находит файл, **не создавайте ссылку**: найдите Laravel командой `find ~ -maxdepth 6 -type f -path '*/backend/public/index.php' -print`. Относительная цель `stepology.ru/backend/public` из каталога `~/stepology.ru` неверна: она указывает на `~/stepology.ru/stepology.ru/backend/public`.

Если вы **уже разместили Laravel внутри `public_html`**, а затем переименовали каталог в `public_html.before-step`, приложение теперь лежит в `public_html.before-step`. Создайте ссылку на его фактический `public`, найдя корень по файлу `artisan`:

```sh
cd ~/stepology.ru
ARTISAN=$(find "$PWD/public_html.before-step" -maxdepth 5 -type f -name artisan -print -quit)
if [ -n "$ARTISAN" ] && [ -f "$(dirname "$ARTISAN")/public/index.php" ]; then
  ln -s "$(dirname "$ARTISAN")/public" public_html
  readlink public_html
  ls -l public_html/index.php public_html/app/index.html public_html/admin/index.html
else
  echo 'Не найден каталог Laravel public; проверьте структуру public_html.before-step'
fi
```

Эта команда применима, когда `public_html` уже отсутствует, а `public_html.before-step` сохранён. Она не перемещает и не удаляет код.

В панели Beget выберите PHP 8.3+ и HTTPS для сайта. Для вложений выставьте `upload_max_filesize=6M` и `post_max_size=7M` или больше.

В `project/backend` выполните `pwd`: скопируйте показанный полный путь. Затем в панели **CronTab → Мастер заданий** создайте **одно** задание типа **«Произвольная команда»**. В поле команды вставьте строку ниже, заменив путь на свой (имя каталога сайта может отличаться от домена):

```sh
cd /home/<логин>/<каталог-сайта>/project/backend && /usr/local/php/cgi/8.3/bin/php artisan schedule:run >> storage/logs/cron.log 2>&1
```

Интервал в Мастере заданий — **каждую минуту**. Если панель показывает пять полей расписания, укажите по `*` в минутах, часах, дне месяца, месяце и дне недели (`* * * * *`). Расписание настраивается отдельно: **не вставляйте звёздочки перед командой**. Сохраните и включите задание. Через одну-две минуты проверьте `storage/logs/cron.log`; ручная проверка из SSH: `/usr/local/php/cgi/8.3/bin/php artisan max:tick` из каталога `backend`. Пустые счётчики `0` нормальны. [Подробные шаги и диагностика](BEGET_SHARED_HOSTING.md#6-включить-обработку-бота-через-crontab).

После подключения выбранного домена к Beget и выпуска SSL проверьте по HTTPS `/health/ready`, `/app/`, `/admin/` и вход владельца. В настройках **нового бота** укажите Mini App URL `https://<ваш-домен>/app/`, а через MAX API создайте для него webhook-подписку на `https://<ваш-домен>/integrations/max/webhook` с типами `message_created`, `bot_started`, `message_callback` и **тем же новым секретом** из `.env`. Это отдельная подписка нового бота; подписку старого бота переносить или удалять для запуска нового не нужно. Затем проверьте Mini App и публикацию заявки. Новая база пуста, поэтому исполнитель должен открыть нового бота и заново сохранить профиль, прежде чем получит пуш. Порядок подключения домена и настройки MAX описан в [подробной инструкции](BEGET_SHARED_HOSTING.md#7-подключить-домен-и-переключить-max).

## Обновление из Git

После обновления кода уведомления MAX отправляются сразу после HTTP-ответа на действие. Cron с интервалом **одна минута** остаётся для повторных попыток и остатка очереди. Для уже настроенного сайта `stepology.ru`, где репозиторий лежит в `public_html.before-step`, используйте каталог `/home/b/bsexecrq/stepology.ru/public_html.before-step` вместо примерного `~/step-bot.madebypavel.space/project`.

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
