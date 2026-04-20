# internal/detector

## ¿Qué hace este paquete?

**Detecta automáticamente qué tecnologías** estás usando en un proyecto mirando archivos específicos:
- Go → busca `go.mod`
- Node.js → busca `package.json`
- Python → busca `requirements.txt`, `pyproject.toml`, etc.
- Docker → busca `Dockerfile`
- Y muchos más...

## ¿Por qué es útil?

Cuando instalás skills, `skillsync` puede **recomendarte** las skills relevantes para tu stack. Por ejemplo, si detecta FastAPI, te sugiere la skill de FastAPI.

## Conceptos de Go que vas a ver acá

### 1. Maps (diccionarios)
```go
var techIndicators = map[string][]string{
    "go":     {"go.mod", "go.sum", "main.go"},
    "python": {"requirements.txt", "pyproject.toml"},
}
```
- `map[K]V` → diccionario con claves tipo K y valores tipo V
- `map[string][]string` → claves son strings, valores son slices de strings
- Se declara con **var** porque es una variable del paquete (global al package)

### 2. Glob patterns
```go
matches, _ := filepath.Glob(filepath.Join(dir, "*.go"))
```
- `Glob()` busca archivos que coincidan con un patrón
- `*.go` → todos los archivos que terminan en `.go`
- Devuelve un slice con los paths que coinciden
- El `_` ignora el segundo valor de retorno (el error) — NO hagas esto en producción, es solo para código simple

### 3. Range sobre maps
```go
for tech, indicators := range techIndicators {
    // tech = "go", indicators = ["go.mod", "go.sum", ...]
}
```
- `range` itera sobre colecciones (slices, maps, arrays)
- Para maps: primera variable es la clave, segunda es el valor
- Si solo querés las claves: `for tech := range techIndicators`

### 4. Nested loops con break
```go
for tech, indicators := range techIndicators {
    for _, indicator := range indicators {
        if found {
            break // Sale solo del loop interno
        }
    }
}
```
- `break` sale del loop más interno
- Si querés salir de todos los loops, necesitás un `label` o un `return`

## Función principal: Detect()

```go
func Detect(dir string) []string {
    var detected []string
    seen := make(map[string]bool)
    
    for tech, indicators := range techIndicators {
        for _, indicator := range indicators {
            matches, _ := filepath.Glob(filepath.Join(dir, indicator))
            if len(matches) > 0 {
                if !seen[tech] {
                    detected = append(detected, tech)
                    seen[tech] = true
                }
                break
            }
        }
    }
    
    return detected
}
```

### ¿Qué hace?

1. **Inicializa variables:**
   - `detected`: slice vacío para guardar las tecnologías encontradas
   - `seen`: map para evitar duplicados

2. **Itera sobre cada tecnología:**
   - Para cada tech (ej: "go"), revisa cada indicador (ej: "go.mod")
   - Usa `Glob()` para buscar el archivo en el directorio

3. **Si encuentra un match:**
   - Verifica que no esté ya detectado (`!seen[tech]`)
   - Agrega la tech al slice con `append()`
   - Marca como vista en el map
   - Hace `break` (no necesita seguir buscando otros indicadores)

4. **Devuelve el slice de tecnologías detectadas**

## Detección avanzada: detectFromContent()

```go
func detectFromContent(dir string, seen map[string]bool) []string {
    var extra []string
    
    pkgJSON := filepath.Join(dir, "package.json")
    if data, err := os.ReadFile(pkgJSON); err == nil {
        content := string(data)
        if contains(content, `"react"`) {
            extra = append(extra, "react")
        }
    }
    
    return extra
}
```

### ¿Por qué?

Algunas tecnologías **no se pueden detectar solo por nombre de archivo**:
- React → necesitás mirar dentro de `package.json` y ver si tiene `"react"` como dependencia
- FastAPI → necesitás ver si `requirements.txt` tiene la línea `fastapi`

### Patrón: "Si existe el archivo, leelo"
```go
if data, err := os.ReadFile(file); err == nil {
    // El archivo existe y se pudo leer
    content := string(data)
    // hacer algo con el contenido
}
```
- En Go, declarás variables dentro del `if`
- Si `err == nil`, significa que NO hubo error (archivo leído OK)
- Convertís `[]byte` a `string` para poder buscar texto

## Función auxiliar: contains()

```go
func contains(s, substr string) bool {
    for i := 0; i <= len(s)-len(substr); i++ {
        if s[i:i+len(substr)] == substr {
            return true
        }
    }
    return false
}
```

**Implementación manual de búsqueda de substring:**
- Recorre el string `s` carácter por carácter
- En cada posición, compara una "ventana" del tamaño de `substr`
- Si encuentra match, devuelve `true`
- Si termina el loop sin encontrar nada, devuelve `false`

**Nota:** Esto se podría simplificar con `strings.Contains()` de la stdlib, pero está implementado manualmente (quizás para aprender o por requisitos específicos).

## Conceptos clave

- **Maps**: Búsqueda O(1) para evitar duplicados
- **append()**: Agrega elementos a un slice (como `push` en JS)
- **filepath.Glob()**: Búsqueda de archivos con patrones
- **os.ReadFile()**: Lee un archivo completo en memoria
- **Type conversion**: `string([]byte)` convierte bytes a texto
- **Early return/break**: Optimización para no seguir buscando cuando ya encontraste

---

**Tip para testear:**
```bash
cd /tu/proyecto
go run cmd/skillsync/main.go
# Vas a ver las tecnologías detectadas en el wizard
```
