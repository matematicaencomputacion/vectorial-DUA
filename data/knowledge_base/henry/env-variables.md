# Variables de entorno (`.env`)

En Henry y en proyectos Node/Python, las **variables de entorno** guardan secretos y configuración fuera del código fuente.

## Qué es un `.env`

Un archivo `.env` es una lista de pares `CLAVE=valor`. No se sube a git. Se carga al iniciar la app (por ejemplo con `dotenv`).

## Ejemplo

```bash
DATABASE_URL=postgres://user:pass@localhost:5432/henry
PORT=3001
```

## Buenas prácticas

- Nunca hardcodees passwords en el código.
- Usa nombres en MAYÚSCULAS_CON_GUIONES.
- Documenta las claves requeridas en el README del proyecto.
