# Keycloak на подпути - Финальная конфигурация

Keycloak развертывается на **подпути `/auth`** общего домена, а не на отдельном поддомене.

##  URL-адреса

```
Keycloak:        https://yourdomain.com/auth/
Admin Console:   https://yourdomain.com/auth/admin/
Realms:          https://yourdomain.com/auth/realms/my-realm
Account:         https://yourdomain.com/auth/realms/my-realm/account/
Health:          https://yourdomain.com/auth/health/ready
```

### Nginx конфигурация

**Критически важно: `/` в конце proxy_pass!**

```nginx
server {
    listen 443 ssl http2;
    server_name yourdomain.com;

    # SSL сертификаты
    ssl_certificate /etc/letsencrypt/live/yourdomain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/yourdomain.com/privkey.pem;

    # Keycloak на /auth
    location /auth/ {
        # ВАЖНО: / в конце обязателен!
        proxy_pass http://127.0.0.1:8080/;
        
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-Host $host;
        proxy_set_header X-Forwarded-Port $server_port;
        
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }

    # Backend API (если есть)
    location /api/ {
        proxy_pass http://127.0.0.1:38080/;
        # ... headers
    }

    # Frontend (если есть)
    location / {
        root /var/www/frontend;
        try_files $uri $uri/ /index.html;
    }
}
```
