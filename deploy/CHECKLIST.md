# Чек-лист для настройки автоматического деплоя

## ☑️ Перед началом

- [ ] У вас есть VPS сервер с Ubuntu/Debian
- [ ] На VPS установлен Docker и Docker Compose
- [ ] У вас есть доступ к Selectel Container Registry
- [ ] У вас есть права администратора в GitHub репозитории

## 1️⃣ Настройка VPS

### Установка Docker (если не установлен)

```bash
# На VPS
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh
```

### Создание структуры директорий

```bash
# На VPS
mkdir -p /root/otus-microservice/prod/be/{configs,logs,data/files}
chmod -R 755 /root/otus-microservice/prod/be
```

### Создание конфига

```bash
# На VPS
nano /root/otus-microservice/prod/be/configs/config.prod.yaml
```

Используйте пример из `configs/config.prod.example.yaml`

**Проверка:**
- [ ] Docker установлен: `docker --version`
- [ ] Docker Compose установлен: `docker compose version`
- [ ] Директории созданы: `ls -la /root/otus-microservice/prod/be/`
- [ ] Конфиг создан: `cat /root/otus-microservice/prod/be/configs/config.prod.yaml`

## 2️⃣ Настройка SSH

### Создание SSH ключей

```bash
# На локальном компьютере
ssh-keygen -t ed25519 -C "github-actions-deploy" -f ~/.ssh/vps_deploy
```

### Копирование ключа на VPS

```bash
# На локальном компьютере
ssh-copy-id -i ~/.ssh/vps_deploy.pub root@ВАШ_VPS_IP
```

### Тестирование подключения

```bash
# На локальном компьютере
ssh -i ~/.ssh/vps_deploy root@ВАШ_VPS_IP
```

**Проверка:**
- [ ] SSH ключ создан: `ls ~/.ssh/vps_deploy*`
- [ ] Подключение работает без пароля
- [ ] Можно выполнять команды: `docker ps`

## 3️⃣ Настройка GitHub Secrets

Перейдите: `Settings` → `Secrets and variables` → `Actions` → `New repository secret`

### Обязательные секреты:

- [ ] `VPS_OTUS_HOST` = IP адрес VPS (например: `192.168.1.100`)
- [ ] `VPS_OTUS_USER` = `root`
- [ ] `VPS_OTUS_SSH_KEY` = содержимое файла `~/.ssh/vps_deploy` (весь приватный ключ)
- [ ] `SELECTEL_REGISTRY_OTUS_USERNAME_PROD` = логин в Selectel Registry
- [ ] `SELECTEL_REGISTRY_OTUS_TOKEN_PROD` = токен Selectel Registry

**Проверка:**
- [ ] Все секреты добавлены в GitHub
- [ ] Значения секретов не содержат лишних пробелов/переносов строк

## 4️⃣ Тестирование деплоя

### Локальная проверка

```bash
# На локальном компьютере
git add .
git commit -m "Setup CI/CD"
git push origin main
```

### Мониторинг деплоя

1. Откройте: `https://github.com/ВАШ_USERNAME/OtusMS/actions`
2. Следите за прогрессом workflow
3. Проверьте каждый шаг:
   - [ ] Lint прошел успешно
   - [ ] Tests прошли успешно
   - [ ] Build and push успешен
   - [ ] Deploy to VPS успешен

### Проверка на VPS

```bash
# На VPS
docker ps | grep otus-microservice
docker logs otus-microservice-be-prod

# Проверка API
curl http://localhost:38080/health
```

**Проверка:**
- [ ] Контейнер запущен
- [ ] Логи не содержат ошибок
- [ ] API отвечает на `/health`

## 5️⃣ Настройка Nginx (опционально)

### Установка Nginx

```bash
# На VPS
sudo apt install -y nginx
```

### Конфигурация

```bash
# На VPS
sudo nano /etc/nginx/sites-available/otus-microservice
```

Пример конфига:

```nginx
server {
    listen 80;
    server_name yourdomain.com;

    location / {
        proxy_pass http://localhost:38080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

```bash
# Активируем конфиг
sudo ln -s /etc/nginx/sites-available/otus-microservice /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl reload nginx
```

**Проверка:**
- [ ] Nginx установлен и запущен
- [ ] Конфигурация валидна: `sudo nginx -t`
- [ ] Сайт доступен по HTTP

## 6️⃣ SSL сертификат (опционально)

```bash
# На VPS
sudo apt install -y certbot python3-certbot-nginx
sudo certbot --nginx -d yourdomain.com
```

**Проверка:**
- [ ] SSL сертификат установлен
- [ ] HTTPS работает
- [ ] Автообновление настроено: `sudo certbot renew --dry-run`

## 7️⃣ Мониторинг и алерты

### Healthcheck

```bash
# Добавьте в crontab для мониторинга
*/5 * * * * curl -f http://localhost:38080/health || echo "Service down!" | mail -s "Alert" your@email.com
```

### Просмотр логов

```bash
# На VPS
docker logs -f otus-microservice-be-prod --tail 100
```

**Проверка:**
- [ ] Healthcheck работает
- [ ] Логи доступны и информативны

## 🎉 Готово!

Теперь при каждом пуше в `main`:
1. ✅ Код проверяется линтером
2. ✅ Запускаются тесты
3. ✅ Собирается Docker образ
4. ✅ Образ публикуется в Registry
5. ✅ Деплой на production сервер

## 🚨 В случае проблем

См. [SSH_SETUP.md](SSH_SETUP.md#troubleshooting) для решения типичных проблем.

## 📚 Полезные ссылки

- [Deploy README](README.md) - подробная документация по деплою
- [SSH Setup](SSH_SETUP.md) - настройка SSH
- [GitHub Actions](https://github.com/features/actions) - документация
- [appleboy/ssh-action](https://github.com/appleboy/ssh-action) - SSH action для GitHub

