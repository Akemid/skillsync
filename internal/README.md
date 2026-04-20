# internal

## ¿Qué es esta carpeta?

En Go, la carpeta `internal/` es **especial**:
- Solo el código en el mismo repo puede importarla
- **Nadie de afuera** puede hacer `import "github.com/tuusuario/skillsync/internal/..."`
- Es para código privado de tu aplicación

### ¿Por qué existe esta convención?

Cuando publicas una librería en Go, no querés que otros dependan de tu código interno. Si cambiás algo en `internal/`, no rompes a nadie porque nadie puede usarlo.

```
✅ Puede importar internal/:
   cmd/skillsync/main.go

❌ NO puede importar internal/:
   Otro proyecto en GitHub
```

## Arquitectura del proyecto

```
cmd/skillsync/main.go          ← COORDINADOR (entry point)
         │
         ├─→ config.Load()     ← Lee YAML, devuelve Config
         ├─→ registry.New()    ← Escanea skills, devuelve Registry
         ├─→ detector.Detect() ← Detecta tecnologías
         ├─→ tui.RunWizard()   ← Wizard interactivo
         └─→ installer.Install() ← Crea symlinks
```

## Flujo completo de una instalación

### 1. Usuario ejecuta: `skillsync`

**main.go** se despierta:
```go
func run() error {
    // 1. Cargar config
    cfg, err := config.Load("~/.config/skillsync/skillsync.yaml")
    
    // 2. Inicializar registry
    reg := registry.New(cfg.RegistryPath)
    reg.Discover()  // escanea ~/.agents/skills/
    
    // 3. Detectar herramientas instaladas
    cfg.Tools = tui.DetectInstalledTools(cfg.Tools)
    
    // 4. Lanzar wizard
    result, err := tui.RunWizard(cfg, reg, projectDir)
    
    // 5. Buscar las skills seleccionadas
    skills := reg.FindByNames(result.SelectedSkills)
    
    // 6. Filtrar herramientas seleccionadas
    tools := filterTools(cfg.Tools, result.SelectedTools)
    
    // 7. ¡Instalar!
    results := installer.Install(skills, tools, result.Scope, projectDir)
    
    // 8. Mostrar resultados
    tui.PrintResults(results)
}
```

### 2. ¿Qué hace cada paquete?

| Paquete | Responsabilidad | Entrada | Salida |
|---------|----------------|---------|--------|
| `config` | Leer/escribir configuración | Path del YAML | `*Config` |
| `registry` | Escanear y buscar skills | Path del registry | `[]Skill` |
| `detector` | Detectar tecnologías | Directorio | `[]string` (techs) |
| `installer` | Crear/borrar symlinks | Skills + Tools + Scope | `[]Result` |
| `tui` | Interfaz interactiva | Config + Registry | `*WizardResult` |

## Patrón de diseño: Separación de responsabilidades

Cada paquete tiene **una única razón para cambiar**:

- **config**: Si cambia el formato del YAML → solo tocas config.go
- **detector**: Si agregás detección de Ruby → solo tocas detector.go
- **installer**: Si cambia cómo crear symlinks → solo tocas installer.go
- **registry**: Si cambia el formato de SKILL.md → solo tocas registry.go
- **tui**: Si querés otro color → solo tocas tui/wizard.go

Esto hace el código **mantenible** y **testeable**.

## Dependencias entre paquetes

```
config ←┬─ main
        ├─ registry
        ├─ installer
        └─ tui

registry ←┬─ main
          └─ tui

detector ←─ tui

installer ←─ main

tui ←─ main
```

### Reglas:
- `main` puede importar a todos
- Los paquetes `internal/*` NO se importan entre sí (salvo config)
- `config` es la única dependencia compartida

**¿Por qué?**
- Evita dependencias circulares
- Hace el código más fácil de entender
- Cada paquete puede evolucionar independientemente

## Patrón: Constructor + Métodos

Todos los paquetes siguen el mismo patrón:

```go
// 1. Definir un struct
type Registry struct {
    BasePath string
    Skills   []Skill
}

// 2. Constructor que devuelve puntero
func New(basePath string) *Registry {
    return &Registry{BasePath: basePath}
}

// 3. Métodos con receiver
func (r *Registry) Discover() error {
    // r es el "this"
}
```

**Ventajas:**
- Encapsulación (los datos están juntos)
- Reusabilidad (podés tener múltiples instancias)
- Testing (podés mockear fácilmente)

## Manejo de errores: Envolver vs Devolver

### Envolver (wrapping)
```go
if err != nil {
    return fmt.Errorf("loading config: %w", err)
}
```
- `%w` preserva el error original
- Podés hacer `errors.Is(err, os.ErrNotExist)` después
- Construye un stack trace de errores

### Devolver directo
```go
if err != nil {
    return err
}
```
- Solo cuando no tenés contexto adicional que agregar

### Cuándo usar cada uno

**Envolver** cuando:
- Querés agregar contexto ("parsing SKILL.md: invalid YAML")
- El error puede atravesar varias capas

**Devolver directo** cuando:
- El error es claro por sí mismo
- Estás en la última capa

## Testing (aunque no hay tests en este repo todavía)

La estructura actual hace el testing fácil:

```go
// Testear config
func TestLoadConfig(t *testing.T) {
    cfg, err := config.Load("testdata/config.yaml")
    // assertions
}

// Testear registry
func TestDiscoverSkills(t *testing.T) {
    reg := registry.New("testdata/skills")
    err := reg.Discover()
    // assertions
}

// Testear detector
func TestDetectGo(t *testing.T) {
    techs := detector.Detect("testdata/go-project")
    // debe contener "go"
}
```

Cada paquete se puede testear **independientemente**.

## Convenciones de Go que usamos

### 1. Errores como valores
```go
func DoSomething() error
func GetSomething() (*Thing, error)
```
- No hay excepciones
- Errores se devuelven explícitamente
- El caller decide qué hacer

### 2. Punteros para structs grandes
```go
func New() *Registry  // devuelve puntero
func (r *Registry) Method()  // receiver puntero
```
- Evita copiar structs grandes
- Permite modificar el struct

### 3. Capitalización = visibilidad
```go
type Registry struct {
    BasePath string   // ✅ Público (exportado)
    skills   []Skill  // ❌ Privado (no exportado)
}
```

### 4. Package names
```go
import "github.com/user/repo/internal/config"

cfg := config.Load()  // se usa como config.Function()
```
- El nombre del package es el último segmento del path
- Debe ser corto y descriptivo

## Próximos pasos para aprender más

### 1. Lee los READMEs en orden:
1. `cmd/skillsync/README.md` — Entry point
2. `internal/config/README.md` — Configuración
3. `internal/registry/README.md` — Gestión de skills
4. `internal/detector/README.md` — Detección de tech
5. `internal/installer/README.md` — Creación de symlinks
6. `internal/tui/README.md` — Interfaz interactiva

### 2. Juega con el código:
```bash
# Ver qué skills hay
go run cmd/skillsync/main.go list

# Ver estado actual
go run cmd/skillsync/main.go status

# Ejecutar el wizard
go run cmd/skillsync/main.go
```

### 3. Modifica algo simple:
- Agrega un nuevo indicador en `detector.go`
- Cambia los colores en `tui/wizard.go`
- Agrega un campo al struct `Config`

### 4. Recursos para aprender Go:
- [Tour of Go](https://tour.golang.org/) — Interactivo, 2-3 horas
- [Effective Go](https://go.dev/doc/effective_go) — Convenciones oficiales
- [Go by Example](https://gobyexample.com/) — Ejemplos prácticos

---

**Tip:** Si algo no te queda claro, buscá el concepto en los READMEs de cada carpeta. Están escritos pensando en alguien que está aprendiendo Go.
