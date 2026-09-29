# Развёртывание «ШАГ» на обычном хостинге Beget

Инструкция для текущей версии проекта: **Laravel 13 + MySQL + cron**, существующие React Mini App и кабинет. Docker, Go, PostgreSQL и MinIO на Beget не нужны. Пример рабочего домена — `step-bot.madebypavel.space`. База на Beget создаётся **пустой**: заявки, профили исполнителей и сессии с VPS не переносятся. Для входа в кабинет создаётся новый владелец. После перехода исполнители заново заполняют профиль в Mini App; только после этого им можно отправлять пуши по новым заявкам.

> До завершения проверки **не удаляйте VPS**. На нём остаётся работающая PHP-версия для возврата домена, если перенос не удастся. На Beget будет отдельная база; новые данные с неё автоматически обратно на VPS не попадут.

## 1. Что проверить в аккаунте Beget

В панели Beget нужны: сайт для домена, SSH, MySQL, SSL и CronTab. В разделе «Сайты» выберите для домена **PHP 8.3 или новее**; такую же версию нужно отдельно выбрать для командной строки и cron. Laravel 13 требует PHP 8.3+ и расширения cURL, DOM, Fileinfo, Filter, Hash, Mbstring, OpenSSL, PCRE, PDO, Session, Tokenizer, XML. Проекту дополнительно нужны `pdo_mysql` и `zip`. Официальные требования: [Laravel 13](https://laravel.com/framework/docs/deployment); настройка PHP сайта: [Beget](https://beget.com/ru/kb/manual/sajty).

1. Включите SSH на главной странице панели Beget. Запишите **логин аккаунта** и **имя сервера** из блока технической информации. Подключение с компьютера: `ssh <логин>@<сервер>.beget.tech`. [Инструкция Beget по SSH](https://beget.com/ru/kb/how-to/ssh/kak-podklyuchitsya-po-ssh-iz-windows).
2. Создайте сайт и привяжите к нему `step-bot.madebypavel.space`. Запишите имя созданного каталога сайта в домашнем каталоге аккаунта. В примерах ниже это `~/step-bot.madebypavel.space`; если Beget дал другое имя, замените путь **во всех** командах.
3. В разделе «Статистика → Информация о сервере» проверьте версию MySQL. Проект проверен на **MySQL 8**. У Beget встречается и MySQL 5.7; при 5.7 сначала попросите поддержку разместить сайт на сервере с MySQL 8, затем продолжайте. [Версии и параметры MySQL на Beget](https://beget.com/ru/kb/faq/hosting/bazy-dannyh-mysql).
4. В разделе «MySQL» создайте новую пустую базу и сохраните её имя и пароль. На обычном хостинге имя пользователя обычно **совпадает с именем базы**, а адрес для сайта — `localhost`. Используйте фактические значения из панели. [Управление MySQL](https://beget.com/ru/kb/manual/mysql).
5. Убедитесь, что в панели есть «CronTab» с запуском **каждую минуту**. Beget поддерживает такой интервал. [Настройка CronTab](https://beget.com/ru/kb/manual/crontab).

Если любой из пунктов недоступен на вашем тарифе, сначала выясните это у поддержки Beget. Не переключайте домен на непроверенное окружение.

## 2. Получить файлы из Git

Подключитесь к Beget по SSH и клонируйте основную ветку репозитория внутрь каталога сайта. Laravel при этом остаётся вне `public_html`, а готовые React-сборки, `composer.lock`, сертификат MAX и скрытый `public/.htaccess` приходят вместе с кодом. Node.js и локальная сборка на Beget не нужны. [Короткая инструкция с командами](BEGET_FROM_GIT.md).

```sh
cd ~/step-bot.madebypavel.space
git clone --branch main https://github.com/Farkk/step-bot.git project
cd project/backend
```

Если репозиторий приватный, настройте доступ GitHub по SSH для аккаунта Beget и замените URL клонирования на SSH-адрес. Файлы `.env`, `vendor` и рабочие данные в Git не находятся; они создаются на хостинге.

## 3. Подготовить PHP, зависимости и закрытые каталоги

Подключитесь по SSH к Beget. Далее команды выполняются **на Beget**, не на VPS:

```sh
cd ~/step-bot.madebypavel.space/project/backend
export PATH=/usr/local/php/cgi/8.3/bin/:$PATH
php -v
php -m | grep -Ei 'curl|dom|fileinfo|mbstring|openssl|pdo|pdo_mysql|zip'
```

Beget приводит этот путь для PHP 8.3 в своей [инструкции по Laravel](https://beget.com/ru/kb/how-to/web-apps/ustanovka-php-frejmvorkov). Если фактическая версия ниже 8.3 или отсутствует `pdo_mysql`, сначала исправьте настройку PHP. Настройка PHP в панели для сайта **сама по себе не меняет PHP в SSH и cron**.

Создайте каталоги, которые должны оставаться вне доступа веб-сервера, и дайте своему пользователю право записи:

```sh
mkdir -p storage/app/private storage/framework/cache/data \
  storage/framework/sessions storage/framework/views storage/logs bootstrap/cache
chmod -R u+rwX storage bootstrap/cache
```

Установите Composer 2 в аккаунте, если его ещё нет. На Beget общесерверный Composer может быть старой версии; для проекта нужен локальный Composer 2. Команды из [инструкции Beget по Composer](https://beget.com/ru/kb/how-to/web-apps/instrukcziya-po-ustanovke-composer):

```sh
mkdir -p ~/.local/bin
wget https://getcomposer.org/installer -O ~/.local/composer-setup.php
php ~/.local/composer-setup.php --install-dir="$HOME/.local/bin" --filename=composer
rm ~/.local/composer-setup.php
~/.local/bin/composer --version
```

В каталоге `backend` выполните:

```sh
COMPOSER_MEMORY_LIMIT=-1 ~/.local/bin/composer install \
  --no-dev --prefer-dist --optimize-autoloader --no-interaction
~/.local/bin/composer check-platform-reqs --no-dev
```

Используйте именно `install` с загруженным `composer.lock`; `composer update` при размещении не нужен. Если Composer сообщает о недостающем расширении или недостатке ресурсов тарифа, исправьте это до переключения домена.

## 4. Настроить окружение и пустую MySQL

В каталоге `backend` скопируйте шаблон и откройте `.env` в редакторе:

```sh
cp .env.example .env
chmod 600 .env
nano .env
```

Заполните как минимум следующие строки. Значения в угловых скобках замените своими; пароль базы и токен **не публикуйте в чате и скриншотах**:

```dotenv
APP_NAME=STEP
APP_ENV=production
APP_KEY=
APP_DEBUG=false
APP_URL=https://step-bot.madebypavel.space
APP_LOCALE=ru
APP_FALLBACK_LOCALE=ru

DB_CONNECTION=mysql
DB_HOST=localhost
DB_PORT=3306
DB_DATABASE=<имя_базы_из_Beget>
DB_USERNAME=<имя_пользователя_из_Beget>
DB_PASSWORD="<пароль_базы_из_Beget>"

SESSION_DRIVER=file
CACHE_STORE=file
QUEUE_CONNECTION=sync
FILESYSTEM_DISK=local

MAX_BOT_TOKEN="<токен_действующего_бота>"
MAX_BOT_WEB_APP=se14459976_bot
MAX_WEBHOOK_SECRET=<секрет_действующей_webhook_подписки>
MAX_API_BASE_URL=https://platform-api2.max.ru
```

`MAX_BOT_WEB_APP` — публичное имя **нынешнего** бота, проверенное через API MAX. Для этого проекта оно `se14459976_bot`; если бот сменится, измените значение. Для текущего домена перенесите с VPS **тот же** `MAX_BOT_TOKEN` и `MAX_WEBHOOK_SECRET` из защищённого файла `/opt/apps/step-bot-php/backend/.env`: тогда существующая подписка на `https://step-bot.madebypavel.space/integrations/max/webhook` останется действительной после изменения DNS. Не копируйте весь VPS `.env`: там другие параметры базы и ключ приложения. В файле `backend/resources/certs/max-ca-bundle.pem` уже есть доверенные сертификаты для исходящих запросов к MAX; сохраните его при загрузке.

Создайте **новый** ключ Laravel и пустую схему базы:

```sh
php artisan key:generate --force
php artisan optimize:clear
php artisan migrate --force --no-interaction
php artisan migrate:status
```

При следующих обновлениях `.env` и `APP_KEY` **сохраняйте**: повторная генерация ключа сбросит действующие сессии. Вложения сохраняются в `backend/storage/app/private`; команду `storage:link` для этих приватных файлов не запускайте.

### Создать владельца кабинета

Старая база не переносится, поэтому прежней учётной записи в новой MySQL нет. Создайте владельца (пароль не короче 12 символов). На Beget из каталога `backend`:

```sh
read -r -s ADMIN_INITIAL_PASSWORD
printf '\n'
export ADMIN_INITIAL_PASSWORD
php artisan owner:create 'ШАГ' '<ваша_почта>' '<ваше_имя>'
unset ADMIN_INITIAL_PASSWORD
```

Пароль вводится после первой команды без отображения на экране. Не добавляйте `ADMIN_INITIAL_PASSWORD` в `.env` и не передавайте его в аргументе команды: аргументы могут попасть в историю оболочки. Команду `owner:create` выполняйте **один раз** для пустой базы.

## 5. Открыть наружу только `public`

В Beget сайт обычно имеет каталог `public_html`. Для Laravel он должен указывать на `backend/public`, а весь остальной PHP-код должен оставаться закрытым. Официальная [инструкция Beget по Laravel](https://beget.com/ru/kb/how-to/web-apps/ustanovka-php-frejmvorkov) использует такую символьную ссылку.

Из каталога сайта сначала посмотрите, что сейчас находится в `public_html`:

```sh
cd ~/step-bot.madebypavel.space
ls -ld public_html
ls -la public_html | head
```

Если это **новый пустой сайт**, сохраните исходный каталог под другим именем и создайте ссылку:

```sh
mv public_html public_html.before-step
ln -s project/backend/public public_html
ls -ld public_html
ls -la public_html/.htaccess public_html/app/index.html public_html/admin/index.html
```

Если в `public_html` уже работает другой сайт, **не выполняйте `mv`**: создайте в панели отдельный сайт/каталог для «ШАГ» и повторите шаг в нём. Никогда не копируйте всё содержимое `backend` внутрь `public_html`. Файлы `.env`, `vendor`, `storage` и код приложения не должны открываться по HTTP.

Для файлов до 5 МБ в панели «Сайты → PHP-директивы» установите `upload_max_filesize` не меньше `6M`, `post_max_size` не меньше `7M`. Эти параметры сайт задаёт отдельно от командной строки. [Директивы PHP на Beget](https://beget.com/ru/kb/manual/sajty).

## 6. Включить обработку бота через CronTab

На обычном хостинге бот работает через webhook и короткую задачу cron; постоянный процесс не нужен. В панели Beget откройте **CronTab → новое задание**, выберите запуск **каждую минуту** (`* * * * *`) и укажите команду с вашим настоящим путём к сайту:

```sh
cd /home/<логин>/step-bot.madebypavel.space/project/backend && /usr/local/php/cgi/8.3/bin/php artisan schedule:run >> storage/logs/cron.log 2>&1
```

Если домашний путь у аккаунта другой, узнайте его на Beget командой `pwd` из каталога `backend` и подставьте. Используйте абсолютный путь к PHP: версия сайта не задаёт версию в CronTab. [Правила CronTab Beget](https://beget.com/ru/kb/manual/crontab).

Проверьте задачу вручную из SSH:

```sh
cd ~/step-bot.madebypavel.space/project/backend
/usr/local/php/cgi/8.3/bin/php artisan schedule:list
/usr/local/php/cgi/8.3/bin/php artisan max:tick
```

Пустая очередь должна дать счётчики `inbox`, `replies`, `notifications` со значением `0`. После включения cron проверьте `storage/logs/cron.log`. Не добавляйте второй cron на `max:tick`: его уже запускает `schedule:run`.

## 7. Переключить домен с VPS на Beget

Эту часть выполняйте после подготовки файлов, базы и cron. Для проверки до смены DNS можно открыть технический адрес Beget или выполнить Artisan-команды по SSH, но **вход и запись через временный домен не проверят рабочий сценарий**: защита кабинета сравнивает заголовок `Origin` с `APP_URL`, указанным выше.

1. Выберите короткое время без работы пользователей. На VPS и Beget разные пустые/старые базы; во время распространения DNS часть запросов может попадать на старый адрес. Не создавайте заявки и не регистрируйте исполнителей в этот промежуток.
2. В панели DNS для `step-bot.madebypavel.space` направьте `A`-запись на IP сайта Beget. Если есть `AAAA`-запись, проверьте, что она тоже ведёт на Beget, иначе часть пользователей продолжит попадать на VPS. Если DNS находится у Beget, выполняйте изменение там; если у другого провайдера — в его панели.
3. В панели Beget для домена выпустите и установите бесплатный SSL-сертификат, включите перенаправление HTTP → HTTPS. Beget может менять `A`-запись при выпуске сертификата на своих DNS; при сторонних DNS IP из письма Beget вносится вручную. [SSL на Beget](https://beget.com/ru/kb/how-to/sites/podklyuchenie-ssl-k-sajtu).
4. Дождитесь, пока `https://step-bot.madebypavel.space/health/ready` отвечает с Beget. Проверка `/health/live` должна вернуть JSON `{"status":"live"}`. Затем проверьте `/app/` и `/admin/`.
5. У действующего бота MAX webhook уже подписан на **тот же URL**. Если в `.env` сохранён прежний секрет, повторная подписка не нужна; после обновления DNS MAX станет обращаться к Beget. Убедитесь в настройках бота, что Mini App URL — `https://step-bot.madebypavel.space/app/`, а webhook — `https://step-bot.madebypavel.space/integrations/max/webhook`. HTTPS на порту 443 и `X-Max-Bot-Api-Secret` обязательны для этой конфигурации. [Документация MAX по webhook](https://dev.max.ru/docs-api/methods/POST/subscriptions).
6. Когда новый сайт и бот проверены, остановите **PHP-приложение и scheduler на прежнем VPS**, чтобы не было двух активных обработчиков. Если у вас сохранён SSH-алиас `vps`, команда: `ssh vps 'cd /opt/apps/step-bot-php && docker compose -f compose.php.vps.yaml -f compose.php.vps.traefik.yaml stop app scheduler'`. Старые тома и данные VPS пока оставьте для возможного возврата.

Если сертификат ещё выпускается или DNS расходится, не проверяйте публикацию реальной заявки и не считайте перенос завершённым. При необходимости верните `A`/`AAAA` на VPS и разберите причину до повторной попытки.

## 8. Контрольный проход после переключения

В браузере или терминале проверьте:

```sh
curl -i https://step-bot.madebypavel.space/health/ready
curl -I https://step-bot.madebypavel.space/app/
curl -I https://step-bot.madebypavel.space/admin/
curl -I https://step-bot.madebypavel.space/.env
```

Ожидается `200` для первых трёх адресов и `404`/`403` для `.env`. Затем:

1. Войдите в кабинет **новым владельцем**; создайте пробный черновик, опубликуйте его, убедитесь, что нет `403`.
2. Откройте Mini App из действующего бота MAX и заново сохраните профиль исполнителя. При пустой новой базе до этого момента пушу просто некому отправляться.
3. Опубликуйте ещё одну тестовую заявку и проверьте карточку в чате MAX, кнопку «Откликнуться», открытие Mini App и смену статусов.
4. На Beget выполните `php artisan max:tick` и проверьте, что `notification_outbox` не содержит новых ошибок. Столбец `last_error` можно смотреть в phpMyAdmin Beget. Если сообщение не пришло, сначала проверьте `last_error`, регистрацию исполнителя, `MAX_BOT_WEB_APP`, `MAX_BOT_TOKEN` и работу cron.
5. Проверьте загрузку и скачивание файла до 5 МБ. По `/health/ready` также проверяется доступность MySQL и право записи в приватное хранилище.

API MAX подтверждает отправку отдельно от того, увидел ли пользователь системное уведомление на устройстве. Последнее проверяется в самом MAX.

## 9. Обновления и резервные копии

При последующих обновлениях получите код из Git. **Не перезаписывайте** серверные `.env`, `storage` и `APP_KEY`. Из SSH:

```sh
cd ~/step-bot.madebypavel.space/project
git pull --ff-only origin main
cd backend
export PATH=/usr/local/php/cgi/8.3/bin/:$PATH
COMPOSER_MEMORY_LIMIT=-1 ~/.local/bin/composer install --no-dev --prefer-dist --optimize-autoloader --no-interaction
php artisan migrate --force --no-interaction
php artisan optimize:clear
php artisan max:tick
```

Перед обновлением сохраните копии MySQL и `backend/storage/app/private`; `.env` держите в защищённом месте. Эти три части вместе нужны для восстановления: в БД лежат метаданные вложений, файлы находятся в `storage`, а ключи и токен — в `.env`. Резервное копирование и хранение выбирайте в панели Beget в соответствии с тарифом. После перехода с пустой базой восстановление через старый VPS вернёт **только старое состояние VPS**, а не новые заявки Beget.

## 10. Если что-то не работает

| Симптом | Что проверить |
| --- | --- |
| `500` или белая страница | `storage/logs/laravel.log`, `APP_DEBUG=false`, версия PHP/расширения, `vendor`, право записи в `storage` и `bootstrap/cache`. Ошибки показывайте разработчику без `.env` и токенов. |
| `/health/ready` → `503` | Доступ к MySQL и запись в `storage/app/private`; проверьте `DB_HOST`, имя базы и пароль. |
| `/app/` или `/admin/` → `404` | Ссылка `public_html → backend/public`, наличие `public/.htaccess`, `public/app/index.html`, `public/admin/index.html`, обработка rewrite. |
| В кабинете при записи `403` | `APP_URL` должен в точности совпадать с HTTPS-доменом в браузере; проверьте вход и CSRF, затем `php artisan optimize:clear`. |
| Webhook без ответа / бот молчит | HTTPS и SSL, адрес подписки и секрет MAX, `storage/logs/laravel.log`, CronTab и `storage/logs/cron.log`. |
| Заявка есть, пуша нет | Сначала зарегистрируйте исполнителя в новой Mini App, затем смотрите `notification_outbox.last_error`; проверьте `MAX_BOT_WEB_APP`, CA-файл и исходящее HTTPS-соединение к MAX. |
| Загрузка файла не проходит | Директивы `upload_max_filesize`, `post_max_size`, запись в `storage/app/private`; лимит приложения — 5 МБ. |

Для диагностики на Beget из каталога `backend` используйте `tail -n 100 storage/logs/laravel.log` и `tail -n 100 storage/logs/cron.log`. Не включайте `APP_DEBUG=true` на публичном сайте.
