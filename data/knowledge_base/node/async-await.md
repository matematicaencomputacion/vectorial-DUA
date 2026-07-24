# Node.js: async/await

`async/await` simplifica el trabajo con Promises en JavaScript/Node.

## Función async

Una función marcada con `async` siempre devuelve una Promise. Dentro puedes usar `await` para pausar hasta que la Promise se resuelva.

```js
async function loadUser(id) {
  const res = await fetch(`/api/users/${id}`);
  return res.json();
}
```

## Errores

Envuelve `await` en `try/catch`. Un rechazo de Promise se convierte en excepción.

## Relación con Henry

En backends Express de Henry, los controladores async deben propagar errores al middleware (`next(err)`) o usar wrappers para no dejar requests colgados.
