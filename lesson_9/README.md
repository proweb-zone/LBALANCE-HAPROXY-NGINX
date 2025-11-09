# Балансировка и отказоустойчивость

В ходе работы я развернул и настроил в docker:

- postgresql_master
- postgresql_slave1
- postgresql_slave2
- haproxy // для балансирования нагрузки (на postgresql)
- nginx //  для балансирования нагрузки между инстансами приложения
- app1 // инстанс приложения 1
- app2 // инстанс приложения 2
- app3 // инстанс приложения 3

### Инструкция по развертыванию

Перед запуском убедитесь что у Вас установлен Docker. Если нужно провести нагрузочное тестирование - установите утилиту K6

1) скачайте репозитарий с Github
```
git clone https://github.com/proweb-zone/LBALANCE-HAPROXY-NGINX.git
cd LBALANCE-HAPROXY-NGINX
```
2) Запустите docker
```
docker compose up -d --build
```

### Нагрузочное тестирование

Для запуска нагрузочного тестирования на K6 воспользуейтесь командой

```
K6_WEB_DASHBOARD=true K6_WEB_DASHBOARD_EXPORT=result/html-report.html k6 run load_test/script.js
```


### Конфигурация HaProxy

```
global
    daemon
    maxconn 256

defaults
    mode tcp
    timeout connect 5000ms
    timeout client 50000ms
    timeout server 50000ms
    log global

# Статистика HAProxy (доступна по http://localhost:8404/stats)
listen stats
    mode http
    bind *:8404
    stats enable
    stats uri /stats
    stats refresh 10s

# Балансировка для PostgreSQL
listen postgres_cluster
    mode tcp
    bind *:5433
    balance roundrobin
    option tcp-check
    default-server inter 3s fall 3 rise 2

    # Мастер только для записи
    server postgres-master postgres-master:5432 check port 5432

    # Слейвы для чтения
    server postgres-slave1 postgres-slave1:5432 check port 5432
    server postgres-slave2 postgres-slave2:5432 check port 5432
```

### Конфигурация Nginx

```
events {
    worker_connections 1024;
}

http {
    upstream backend {
        # Балансировка с помощью round-robin (по умолчанию)
        server localhost:3025;  # Порт по умолчанию для Go приложений
        server localhost:3026;
        server localhost:3027;
    }

    server {
        listen 80;

        # Базовые настройки
        client_max_body_size 10m;
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;

        location / {
            proxy_pass http://backend;

            # Заголовки для корректной работы прокси
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;

            # Настройки для keep-alive
            proxy_http_version 1.1;
            proxy_set_header Connection "";
        }

        # Эндпоинт для health checks
        location /health {
            proxy_pass http://backend;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
        }
    }
}
```
### Логи

После запуска нагрузочного тестирования я по очереди отключал postgres_slave1, postgres_slave2 - приложение продолжало работать и исполнять запись в БД

Ссылка на лог работы с отключеннием postgres_slave
https://github.com/proweb-zone/LBALANCE-HAPROXY-NGINX/lesson_9/image.png
