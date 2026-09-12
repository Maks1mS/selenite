<h1 align="center">
  <img src="logo.svg" alt="Selenite" width="128">
  <br>Selenite
</h1>

<p align="center">
  Минималистичная иммутабельная серверная ОС для контейнеров.
  <br>Построена на базе <a href="https://alt-atomic.org">ALT Atomic</a>.
</p>

<!-- [![Quality Gate Status](https://sonar.alt-gnome.ru/api/project_badges/measure?project=selenite&metric=alert_status&token=sqb_425aa0aae9e5551e87bc53bf1881858654f327ca)](https://sonar.alt-gnome.ru/dashboard?id=selenite) -->

## О проекте

Selenite — иммутабельная серверная операционная система, предназначенная для запуска контейнерных нагрузок. Базовая система доступна только для чтения и обновляется атомарно целым образом, а приложения разворачиваются через стандартные стеки Podman Compose.

Подробнее на [seleniteos.ru](https://seleniteos.ru).

### Основные возможности

- **Неизменяемая основа** — базовая система доступна только для чтения и обновляется целым образом.
- **Декларативная конфигурация** — состояние машины описывается в едином YAML и применяется одной командой.
- **Минимальная поверхность атаки** — ограниченный API поверх SSH вместо интерактивного shell, rootless-контейнеры и RBAC.
- **Привычные compose-стеки** — разворачивайте приложения из стандартных `compose.yml`.

### Минимальные требования

- ОЗУ: 256 МБ
- Диск: 4 ГБ

### Документация

См. [документацию](https://seleniteos.ru/guide/intro) на сайте Selenite.

## Лицензия

[MPL-2.0](LICENSE)
