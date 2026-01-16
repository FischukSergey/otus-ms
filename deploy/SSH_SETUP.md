# Настройка SSH для автоматического деплоя

## Шаг 1: Создание SSH ключа

На вашем локальном компьютере:

```bash
# Создаем новую пару ключей специально для GitHub Actions
ssh-keygen -t ed25519 -C "github-actions-deploy" -f ~/.ssh/vps_deploy

# Не устанавливайте passphrase (просто нажмите Enter)
```

## Шаг 2: Копирование публичного ключа на VPS

```bash
# Копируем публичный ключ на сервер
ssh-copy-id -i ~/.ssh/vps_deploy.pub root@YOUR_VPS_IP

# Или вручную:
cat ~/.ssh/vps_deploy.pub
# Скопируйте вывод, затем на VPS:
# mkdir -p ~/.ssh
# echo "ВСТАВЬТЕ_ПУБЛИЧНЫЙ_КЛЮЧ" >> ~/.ssh/authorized_keys
# chmod 600 ~/.ssh/authorized_keys
# chmod 700 ~/.ssh
```

## Шаг 3: Проверка подключения

```bash
# Проверяем что можем подключиться с новым ключом
ssh -i ~/.ssh/vps_deploy root@YOUR_VPS_IP

# Если подключение успешно - отлично!
exit
```

## Шаг 4: Получение приватного ключа для GitHub

```bash
# Выводим приватный ключ
cat ~/.ssh/vps_deploy

# Скопируйте ВСЁ, включая строки:
# -----BEGIN OPENSSH PRIVATE KEY-----
# ...
# -----END OPENSSH PRIVATE KEY-----
```

## Шаг 5: Настройка GitHub Secrets

1. Перейдите в ваш репозиторий на GitHub
2. `Settings` → `Secrets and variables` → `Actions`
3. Нажмите `New repository secret`
4. Создайте следующие секреты:

### VPS_OTUS_HOST
- Name: `VPS_OTUS_HOST`
- Secret: `YOUR_VPS_IP` (например: `192.168.1.100`)

### VPS_OTUS_USER
- Name: `VPS_OTUS_USER`
- Secret: `root`

### VPS_OTUS_SSH_KEY
- Name: `VPS_OTUS_SSH_KEY`
- Secret: Вставьте содержимое файла `~/.ssh/vps_deploy` (приватный ключ)
  ```
  -----BEGIN OPENSSH PRIVATE KEY-----
  b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
  ...
  -----END OPENSSH PRIVATE KEY-----
  ```

### SELECTEL_REGISTRY_OTUS_USERNAME_PROD
- Name: `SELECTEL_REGISTRY_OTUS_USERNAME_PROD`
- Secret: Ваш логин в Selectel Container Registry

### SELECTEL_REGISTRY_OTUS_TOKEN_PROD
- Name: `SELECTEL_REGISTRY_OTUS_TOKEN_PROD`
- Secret: Ваш токен для Selectel Container Registry

## Шаг 6: Подготовка VPS

На вашем VPS выполните:

```bash
# Создаем структуру директорий
mkdir -p /root/otus-microservice/prod/be/{configs,logs,data/files}

# Создаем конфигурационный файл
nano /root/otus-microservice/prod/be/configs/config.prod.yaml
```

Пример конфига (используйте из вашего репозитория):
```yaml
global:
  env: prod

log:
  level: info

servers:
  debug:
    addr: 0.0.0.0:33000
  client:
    addr: 0.0.0.0:38080
    allow_origins:
      - "https://yourdomain.com"
```

## Шаг 7: Проверка

```bash
# Сделайте коммит в main ветку
git push origin main

# Проверьте GitHub Actions:
# https://github.com/YOUR_USERNAME/OtusMS/actions

# На VPS проверьте логи:
ssh root@YOUR_VPS_IP
docker ps
docker logs otus-microservice-be-prod
```

## Безопасность

### Рекомендации:

1. **Отключите root login с паролем** (только SSH ключи):
```bash
# На VPS
sudo nano /etc/ssh/sshd_config

# Измените:
PermitRootLogin prohibit-password
PasswordAuthentication no

# Перезапустите SSH
sudo systemctl restart sshd
```

2. **Используйте firewall**:
```bash
# На VPS с Ubuntu
sudo ufw allow 22/tcp
sudo ufw allow 38080/tcp
sudo ufw allow 33000/tcp
sudo ufw enable
```

3. **Ограничьте доступ по IP** (опционально):
```bash
# Разрешить SSH только с определенных IP
sudo ufw allow from GITHUB_ACTIONS_IP to any port 22
```

## Troubleshooting

### Ошибка: Permission denied (publickey)

```bash
# Проверьте права на файлы
ls -la ~/.ssh/

# Должно быть:
# drwx------  .ssh/
# -rw-------  authorized_keys
# -rw-------  id_ed25519
# -rw-r--r--  id_ed25519.pub
```

### Ошибка: Host key verification failed

```bash
# Добавьте VPS в known_hosts
ssh-keyscan YOUR_VPS_IP >> ~/.ssh/known_hosts
```

### Проверка логов на VPS

```bash
# Логи SSH
sudo tail -f /var/log/auth.log

# Логи Docker
docker logs otus-microservice-be-prod -f
```

## Полезные команды

```bash
# Тестирование SSH подключения
ssh -v -i ~/.ssh/vps_deploy root@YOUR_VPS_IP

# Просмотр публичного ключа
cat ~/.ssh/vps_deploy.pub

# Просмотр приватного ключа (НИКОГДА НЕ ДЕЛИТЕСЬ ИМ!)
cat ~/.ssh/vps_deploy

# Удаление ключа с сервера
ssh root@YOUR_VPS_IP "sed -i '/github-actions-deploy/d' ~/.ssh/authorized_keys"
```

