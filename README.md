# climacep-otel

Este projeto consiste em dois microserviços desenvolvidos em Go que recebem um CEP, identificam a cidade correspondente, obtêm a temperatura atual (em Celsius, Fahrenheit e Kelvin) e retornam essas informações. O projeto inclui rastreamento distribuído usando OpenTelemetry e Zipkin.


## Estrutura do Projeto


```plaintext
climacep-otel/
├── serviceA/
│   ├── Dockerfile
│   ├── go.mod
│   ├── go.sum
│   ├── main.go
├── serviceB/
│   ├── Dockerfile
│   ├── go.mod
│   ├── go.sum
│   ├── main.go
├── docker-compose.yml
├── README.md
```


## Serviços

### Serviço A
- Porta: `8080`
- Função: Recebe o CEP via POST, valida o formato e, se válido, encaminha o CEP para o Serviço B via HTTP.


### Serviço B
- Porta: `8081`
- Função: Recebe o CEP, consulta o serviço ViaCEP para encontrar a cidade, obtém a temperatura da cidade e retorna os dados formatados.


## Rastreio Distribuído


OpenTelemetry com exportação para Zipkin é usado para rastrear requests entre os serviços.


## Como Rodar o Projeto

### Pré-requisitos
- Docker e Docker Compose 


### Passos Para Rodar
1. Clone o repositório:

```sh
git clone https://github.com/walterlicinio/climacep-otel
cd climacep-otel
```

2. Construa e inicie os containers:

```sh
docker-compose up --build
```

Esse comando irá subir os serviços A, B juntamente com o serviço de rastreamento Zipkin.


3. Acesse a interface do Zipkin em [http://localhost:9411](http://localhost:9411) para visualizar os traces.



### Testando o serviço



Você pode testar o Serviço A com um CEP válido usando uma ferramenta como CURL ou Postman:



```sh

curl -X POST http://localhost:8080 -d '{"cep": "58045040"}' -H "Content-Type: application/json"

```



### Exemplo de Resposta

Em caso de sucesso:
```json
{
    "city": "São Paulo",
    "temp_C": 28.5,
    "temp_F": 83.3,
    "temp_K": 301.5
}
```


Em caso de erro:
- CEP inválido (menos de 8 dígitos, ou contém não numérico):
  - Código HTTP: `422`
  - Resposta: `invalid zipcode`

- CEP não encontrado:
  - Código HTTP: `404`
  - Resposta: `can not find zipcode`