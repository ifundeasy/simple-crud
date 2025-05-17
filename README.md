# simple-crud

hello world
```bash
curl --location 'http://localhost:3000'
```

create product
```bash
curl --location 'http://localhost:3000/product' --header 'Content-Type: application/json' \
--data '{
    "name": "sirop tjampolay",
    "price": 1000,
    "stock": 100
}'
```

get products
```bash
curl --location --request GET 'http://localhost:3000/products' --header 'Content-Type: application/json'
```

get product by id
```bash
curl --location --request GET 'http://localhost:3000/product?id=6827ac8dbe36af32d9761dd5' --header 'Content-Type: application/json'
```

get products (from external)
```bash
curl --location --request GET 'http://localhost:3000/external' --header 'Content-Type: application/json'
```