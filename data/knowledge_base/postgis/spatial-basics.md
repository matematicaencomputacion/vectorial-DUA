# PostGIS: conceptos espaciales básicos

PostGIS extiende PostgreSQL con tipos y funciones geoespaciales.

## Geometría vs Geografía

- **geometry**: plano cartesiano, rápido para análisis local.
- **geography**: elipsoide WGS84, distancias en metros sobre la Tierra.

## Función clave: `ST_DWithin`

`ST_DWithin(geom_a, geom_b, distancia)` responde si dos geometrías están a menos de una distancia dada. Ideal para búsquedas de cercanía (por ejemplo, cafeterías a 500 m).

## SRID

Usa SRID 4326 (WGS84) para lat/lon. Transforma con `ST_Transform` cuando necesites un sistema proyectado para mediciones planas.
