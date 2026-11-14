# HW2: Стратегии устойчивости

Модельная Go-библиотека для выполнения запросов к внешнему сервису в условиях
ошибок, таймаутов и нескольких реплик. Проект выполнен для курса DDIA.

## Требования

- Go 1.27.2;
- GNU Make.

## Запуск проверок

```bash
make test
make vet
make check
```

`make check` запускает unit-тесты и статическую проверку `go vet`.

## Основная модель

Внешний сервис вызывается через интерфейс `SubrequestSender`. Он получает
`context.Context` и `Request`, а возвращает нормализованный `SubrequestResult`:

- `Success` — успешный ответ (2xx) с payload;
- `ClientFailure` — ошибка 4xx;
- `ServerFailure` — ошибка 5xx;
- `Timeout` — ожидание остановлено по deadline или отмене контекста;
- `Unknown` — статус вне модели задания.

Клиентские deadline передаются через `context.Context`; sender обязан завершить
ожидание при отмене context.

## Бюджеты

`LatencyBudget`:

- `RequestTimeout` — общий лимит времени логического запроса;
- `SubrequestTimeout` — лимит ожидания одной попытки.

`RetryBudget` содержит число **дополнительных повторов**, разрешённых после
соответствующего исхода:

- `ClientFailure` — после 4xx;
- `ServerFailure` — после 5xx;
- `Timeouts` — после timeout.

Например, `ServerFailure: 1` допускает последовательность `5xx → Success`:
после первого 5xx выполняется одна дополнительная попытка.

## Реализованные стратегии

| Стратегия | Поведение |
|---|---|
| `onetry` | Ровно один подзапрос с request и subrequest deadline. |
| `retry` | Последовательные повторы с раздельными retry-бюджетами. |
| `backoff` | Retry с экспоненциальной задержкой после 5xx и timeout. |
| `roundrobin` | Каждый следующий retry отправляется следующему sender-у по кругу. |
| `hedging` | Первый sender запускается сразу; остальные — параллельно после `hedgingDelay`, если первый ещё не ответил. |
| `circuitbreaker` | Исключает sender с высокой долей ошибок в скользящем окне на время cooldown. |

## Структура проекта

```text
internal/resilience/
  request.go, result.go, status.go, budget.go  # базовые типы
  sender.go                                    # интерфейс внешнего сервиса
  strategies/
    onetry/
    retry/
    backoff/
    roundrobin/
    hedging/
    circuitbreaker/
  testutil/                                    # ScriptedSender и FakeSleeper
```

`ScriptedSender` задаёт сценарии исходов и задержек для тестов. `FakeSleeper`
записывает backoff-задержки без реального ожидания.

## Circuit breaker

Для каждого sender-а хранится скользящее окно последних `WindowSize` исходов.
Если доля `ClientFailure`, `ServerFailure` и `Timeout` достигает
`FailureThreshold`, sender исключается из round-robin до `Cooldown`.
После cooldown sender возвращается в ротацию с очищенной историей.
