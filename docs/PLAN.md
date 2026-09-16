---
PLAN: "fix(server): AddTool valida por Access, no con un Action!=0 ciego — una tool solo-autenticada es legítima"
EXECUTOR: jules
REVIEWER: none
STATUS: review
SESSION: 18244365744164825945
PR: https://github.com/webtyp/mcp/pull/29
---

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.

# PLAN — `AddTool` rechaza tools legítimas solo-autenticadas

## 1. El defecto

`server.go`, en `AddTool`:

```go
func (s *Server) AddTool(tool Tool) error {
	if tool.Name == "" || tool.Action == 0 || tool.Execute == nil {
		return fmt.Err("mcp", "invalid tool: Name, Action and Execute are required")
	}
	...
}
```

`tool.Action == 0` se rechaza **siempre**. Pero este mismo paquete, en
`harvest.go`, construye tools con `Action == 0` a propósito:

```go
func (rt *opRoute) Authenticated() router.Route {
	rt.owner.tools[rt.idx].Access = model.AccessAuthenticated
	return rt
}
```

`Authenticated()` declara «hace falta identidad, ninguna acción CRUD sobre un
recurso» y **no toca `Action`**, que es correcto: no hay recurso sobre el que
actuar. `Requires(resource, action)` es el único que asigna `Action`.

**Consecuencia:** cualquier operación registrada con `.Authenticated()` y sin
`.Requires(...)` se cosecha bien y después es rechazada al montarse. La
operación de identidad de `webtyp.com/auth` (`auth.OpMe`, el «¿quién soy?»)
tiene exactamente esa forma — o sea que la librería de autenticación no puede
montar su propia operación a través de su propio transporte.

Esto se descubrió porque una app tuvo que envolver su `ToolProvider` para
parchear el `Action` a mano antes de que `AddTool` lo viera. Envolver una
librería para corregir su comportamiento está prohibido en este ecosistema
([CONSTRUCTION_HARNESS](https://github.com/webtyp/app-releases/blob/main/docs/CONSTRUCTION_HARNESS.md)):
*«Never wrap a library to fix its behaviour. A wrapper that patches a defect is
a fork with a friendlier name. Fix it where it lives and publish.»* Este plan es
ese arreglo.

## 2. Qué hay que entender antes de tocar nada

`model.Access` tiene tres valores y el **cero es el cerrado**:

| `Access` | Significado | `Resource` | `Action` |
|---|---|---|---|
| `model.AccessGuarded` (**valor cero**) | identidad **y** permiso sobre un recurso | **obligatorio** | **obligatorio** |
| `model.AccessAuthenticated` | basta con identidad | debe estar vacío | **debe ser cero** |
| `model.AccessPublic` | sin identidad | debe estar vacío | **debe ser cero** |

Las dos últimas filas son lo que hoy no se puede expresar.

`AddTool` ya valida bien la coherencia `Access`↔`Resource` (el `switch` que
sigue al `if`). Lo que falta es la coherencia `Access`↔`Action`, y que el `if`
ciego deje de pisarla.

**Esto NO debe debilitarse — es el principio 8 del harness, «closed by
default»:** una `Tool` construida a mano sin declarar nada tiene `Access == 0 ==
AccessGuarded` y `Resource == ""`, y debe seguir siendo un error
(`"is guarded but declares no Resource — it would deny every call"`). Escribir
nada sigue dando el estado seguro; abrir sigue costando una línea explícita.

## 3. El cambio

### 3.1 `server.go` — `AddTool`

Sustituir el `if` ciego por la validación por `Access`. La forma exacta:

```go
func (s *Server) AddTool(tool Tool) error {
	if tool.Name == "" || tool.Execute == nil {
		return fmt.Err("mcp", "invalid tool: Name and Execute are required")
	}

	// Access decide qué más hace falta. El cero es AccessGuarded, así que una
	// Tool que no declara nada cae en la rama más estricta.
	switch tool.Access {
	case model.AccessGuarded:
		// Una tool guarded sin recurso autorizaba contra "", lo que denegaba
		// toda llamada: parecía protegida y era inalcanzable, en silencio.
		if tool.Resource == "" {
			return fmt.Err("mcp", "tool", tool.Name, "is guarded but declares no Resource — it would deny every call")
		}
		// Y sin acción no hay permiso con el que comparar: mismo silencio.
		if tool.Action == 0 {
			return fmt.Err("mcp", "tool", tool.Name, "is guarded but declares no Action — no permission could ever match it")
		}
	default:
		// AccessAuthenticated / AccessPublic: nadie comprueba recurso ni
		// acción, así que declararlos es una protección aparente que no
		// existe.
		if tool.Resource != "" {
			return fmt.Err("mcp", "tool", tool.Name, "declares Resource", string(tool.Resource),
				"but its Access does not check it — remove one or the other")
		}
		if tool.Action != 0 {
			return fmt.Err("mcp", "tool", tool.Name, "declares Action", tool.Action.String(),
				"but its Access does not check it — use Requires(resource, action) to guard it, or drop the Action")
		}
	}

	s.mu.Lock()
	s.tools[tool.Name] = tool
	s.mu.Unlock()
	...
}
```

El `switch` que ya existía queda absorbido: es el mismo `switch`, con las dos
comprobaciones de `Action` añadidas a cada rama. **No dejar dos `switch` sobre
`tool.Access` seguidos.**

### 3.2 `tools.go` — el comentario del campo miente

```go
	Action      model.Action   // required — model.Create/Read/Update/Delete
```

pasa a

```go
	// Action es obligatoria con AccessGuarded (es la mitad del permiso que se
	// comprueba) y debe ser cero con AccessAuthenticated/AccessPublic, donde
	// no hay recurso sobre el que actuar. Ver AddTool.
	Action      model.Action
```

### 3.3 `actionByte()` ya tolera el cero

`Tool.actionByte()` devuelve `0` cuando `Action.String()` está vacío. Verificar
que la ruta de ejecución de una tool con `Action == 0` llega a
`Validate(action byte)` con `0` y que eso es aceptable en los modelos generados
por `ormc`; si algún punto asume un byte CRUD válido, **no lo cambies**:
documenta el hallazgo en el PR. No entra en el alcance de este plan.

## 4. El test que hace válido el arreglo

La regla del ecosistema: *«An API is not published until a consumer-shaped test,
inside the library itself, proves it.»* Un test que llame a `AddTool` con un
`Tool` literal **no sirve**: el defecto aparece en el camino real
`OperationModule → HarvestOps → NewServer → AddTool`.

Añadir en `harvest_test.go` (o `server_test.go`, donde encaje con el estilo
existente) un test que:

1. Declare un `router.OperationModule` de prueba con **dos** operaciones:
   - una con `.Authenticated()` y nada más (la forma de `auth.OpMe`),
   - una con `.Requires(model.Resource("thing"), model.Read)`.
2. Las cosecha con `HarvestOps` y construye un `Server` con `NewServer`.
3. Afirma que **las dos** quedaron registradas y son invocables.

Hoy ese test falla en el paso 2 con `invalid tool: Name, Action and Execute are
required`. Ese fallo, antes del arreglo, es la prueba de que el test es el
correcto — escríbelo primero y compruébalo.

Añadir además los tests de rechazo, uno por rama:

| Caso | Esperado |
|---|---|
| `Tool{Name, Execute}` sin nada más (Access cero = Guarded, Resource "") | error: `is guarded but declares no Resource` |
| Guarded con `Resource` y `Action == 0` | error: `is guarded but declares no Action` |
| `AccessAuthenticated` con `Resource != ""` | error: `does not check it` |
| `AccessAuthenticated` con `Action != 0` | error: `does not check it` |
| `AccessAuthenticated` limpia | **aceptada** |
| `AccessPublic` limpia | **aceptada** |

## 5. Criterios de aceptación

- [ ] `gotest ./...` verde en el repo.
- [ ] Existe el test consumidor de §4 y pasa.
- [ ] Los seis casos de la tabla de §5 están cubiertos.
- [ ] Un `Tool{}` vacío sigue siendo un error (closed by default intacto).
- [ ] No queda ningún `switch tool.Access` duplicado en `AddTool`.
- [ ] El comentario de `Tool.Action` describe la regla real.
- [ ] Ningún cambio en `harvest.go`: `Authenticated()` ya era correcto.

## 6. Fuera de alcance

- No tocar `opRegistry.Operation`, ni su pánico por nombre duplicado. Es un
  defecto distinto, con su propio plan (namespacing del nombre de op).
- No añadir campos nuevos a `Tool`.
- No tocar `webtyp.com/auth`.
