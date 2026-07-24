# Proposal: Motor de Ruteo Vectorial DUA y Enclave Adaptativo

## Problema

Los entornos tradicionales de aprendizaje en programación operan como rutas lineales estáticas que no contemplan las lagunas de conocimiento, la tolerancia a la frustración ni los diferentes estilos de aprendizaje (DUA / Carl Rogers). Esto provoca bloqueos cuando el estudiante intenta copiar o adaptar código sin comprender sus fundamentos básicos.

## Objetivo

Implementar un servicio en Go de alta eficiencia y baja latencia ($t < 15\text{ms}$) que reciba el vector de estado del estudiante y su duda actual, calcule la distancia vectorial en un espacio de $N$ dimensiones y devuelva la coordenada/URL del nodo pedagógico DUA más adecuado (estático preexistente) o dispare la generación de una "Estación en Vivo".

## Alcance incluido

- Definición de especificaciones Protobuf (`node_schema.proto`, `student_state.proto`, `router_api.proto`, `events.proto`).
- Motor de búsqueda $k\text{-NN}$ en memoria en Go usando distancia coseno.
- Esquema de nombramiento e indexación de nodos mediante ULID (`dua::<dimension>::<dificultad>::<formato>::<ulid>`).
- Endpoint gRPC para consultas en tiempo real desde el Agente/IDE.
- Evento de fallback (`NodeNotFound`) cuando la distancia coseno mínima no alcance el umbral de similitud.

## Fuera de alcance

- Interfaz gráfica final de la Pantalla Master (se entrega API/Contrato de eventos).
- Sintetizador LLM de nodos en vivo (solo se implementa el evento emisor).
- Persistencia histórica prolongada en base de datos relacional en esta fase.

## Riesgos

- Elevada latencia en la resolución de la distancia coseno si el árbol $k\text{-NN}$ escala en memoria sin estructuración adecuada.
- Degradación de la precisión en el mapeo si el vector de estado del estudiante no normaliza correctamente sus dimensiones.

## Plan de rollback

- Si el servicio en Go no responde en $< 15\text{ms}$, el Agente del IDE hará fallback temporal a respuestas estáticas predefinidas mediante HTTP REST.
