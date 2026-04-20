# internal/registry

## ¿Qué hace este paquete?

Maneja el **registro central de skills**. Escanea el directorio `~/.agents/skills/` y crea una lista de todas las skills disponibles con su metadata.

## Estructura de una skill

```
~/.agents/skills/
├── fastapi/
│   ├── SKILL.md          ← Archivo principal (con metadata YAML)
│   ├── scripts/          ← Código ejecutable (opcional)
│   └── references/       ← Docs de referencia (opcional)
└── sdd-init/
    └── SKILL.md
```

## Conceptos de Go que vas a ver acá

### 1. Métodos en structs
```go
type Registry struct {
    BasePath string
    Skills   []Skill
}

func (r *Registry) Discover() error {
    // r es el "receiver" (como this en otros lenguajes)
    entries, _ := os.ReadDir(r.BasePath)
    // ...
}
```
- `(r *Registry)` es el **receiver** — hace que `Discover()` sea un método de `Registry`
- `*Registry` es un **puntero receiver** (puedes modificar el struct)
- Se llama con: `reg.Discover()`

### 2. Constructor pattern
```go
func New(basePath string) *Registry {
    return &Registry{
        BasePath: config.ExpandPath(basePath),
    }
}
```
- Go NO tiene constructores nativos
- Por convención, se crea una función `New()` que devuelve un puntero al struct
- Inicializa el struct con valores por defecto o transformados

### 3. YAML frontmatter
```yaml
---
name: fastapi
description: FastAPI best practices
---

# FastAPI Skill

Instrucciones en markdown...
```
- El frontmatter es metadata en YAML entre `---`
- El resto del archivo es Markdown normal
- Parseamos solo el frontmatter para extraer metadata

## Structs principales

### Skill
```go
type Skill struct {
    Name        string
    Description string
    Path        string   // Ruta absoluta a la carpeta de la skill
    Files       []string // Lista de archivos dentro
}
```

### SkillMeta
```go
type SkillMeta struct {
    Name        string `yaml:"name"`
    Description string `yaml:"description"`
}
```
- Solo los campos del frontmatter que nos interesan
- El resto del SKILL.md se ignora (es para el AI agent)

## Función: Discover()

```go
func (r *Registry) Discover() error {
    entries, err := os.ReadDir(r.BasePath)
    if err != nil {
        return fmt.Errorf("reading registry %s: %w", r.BasePath, err)
    }
    
    r.Skills = nil // Limpia la lista anterior
    
    for _, entry := range entries {
        if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
            continue // Skip archivos y carpetas ocultas
        }
        
        skillPath := filepath.Join(r.BasePath, entry.Name())
        skillMD := filepath.Join(skillPath, "SKILL.md")
        
        skill := Skill{
            Name: entry.Name(),
            Path: skillPath,
        }
        
        // Intentar leer metadata del frontmatter
        if data, err := os.ReadFile(skillMD); err == nil {
            if meta, err := parseFrontmatter(data); err == nil {
                if meta.Description != "" {
                    skill.Description = meta.Description
                }
            }
        }
        
        // Listar archivos de la skill
        if files, err := listFiles(skillPath); err == nil {
            skill.Files = files
        }
        
        r.Skills = append(r.Skills, skill)
    }
    
    return nil
}
```

### ¿Qué hace?

1. **Lee el directorio base** (`~/.agents/skills/`)
2. **Para cada carpeta:**
   - Skip si no es directorio o empieza con `.`
   - Busca el archivo `SKILL.md`
   - Extrae metadata del frontmatter (name, description)
   - Lista todos los archivos dentro de la skill
   - Agrega la skill al slice `r.Skills`

### Patrón: "Intentar pero no fallar"
```go
if data, err := os.ReadFile(skillMD); err == nil {
    // Si el archivo existe, procesarlo
}
// Si no existe o falla, simplemente continuar
```
- **No es un error fatal** si una skill no tiene `SKILL.md`
- Simplemente no tendrá descripción
- Este patrón es común cuando algo es opcional

## Función: parseFrontmatter()

```go
func parseFrontmatter(data []byte) (*SkillMeta, error) {
    content := string(data)
    
    // Verificar que empieza con ---
    if !strings.HasPrefix(content, "---\n") {
        return nil, fmt.Errorf("no frontmatter found")
    }
    
    // Buscar el cierre del frontmatter
    end := strings.Index(content[4:], "\n---")
    if end < 0 {
        return nil, fmt.Errorf("no frontmatter closing")
    }
    
    // Extraer solo el YAML (sin los ---)
    var meta SkillMeta
    if err := yaml.Unmarshal([]byte(content[4:4+end]), &meta); err != nil {
        return nil, err
    }
    
    return &meta, nil
}
```

### ¿Cómo funciona?

```
---\n
name: fastapi\n
description: FastAPI patterns\n
---\n
# Resto del markdown
```

1. **Verificar inicio:** `strings.HasPrefix(content, "---\n")`
2. **Buscar cierre:** `strings.Index(content[4:], "\n---")`
   - `content[4:]` → empieza después del primer `---\n` (4 caracteres)
3. **Extraer el YAML:**
   - `content[4:4+end]` → desde después del primer `---\n` hasta el segundo `---`
4. **Parsear el YAML:** `yaml.Unmarshal()`

### String slicing en Go
```go
s := "---\nhola\n---"
s[4:]      // "hola\n---" (desde índice 4 hasta el final)
s[4:8]     // "hola" (desde índice 4 hasta 8, exclusivo)
s[:3]      // "---" (desde el inicio hasta 3, exclusivo)
```

## Función: FindByNames()

```go
func (r *Registry) FindByNames(names []string) []Skill {
    nameSet := make(map[string]bool, len(names))
    for _, n := range names {
        nameSet[n] = true
    }
    
    var result []Skill
    for _, s := range r.Skills {
        if nameSet[s.Name] {
            result = append(result, s)
        }
    }
    return result
}
```

### ¿Por qué usar un map como set?

**Sin map (O(n²)):**
```go
for _, skill := range r.Skills {
    for _, name := range names {
        if skill.Name == name {
            result = append(result, skill)
        }
    }
}
```

**Con map (O(n)):**
```go
nameSet := map[string]bool{"fastapi": true, "sdd-init": true}
for _, skill := range r.Skills {
    if nameSet[skill.Name] {
        result = append(result, skill)
    }
}
```

- Crear el set: O(n)
- Buscar cada skill: O(1) por el map
- Total: O(n) vs O(n²)

## Función: listFiles()

```go
func listFiles(dir string) ([]string, error) {
    var files []string
    err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }
        if !info.IsDir() {
            rel, _ := filepath.Rel(dir, path)
            files = append(files, rel)
        }
        return nil
    })
    return files, err
}
```

### filepath.Walk() — Recorrer árbol de directorios

- Visita **recursivamente** cada archivo y carpeta
- Para cada item, llama tu función callback
- El callback recibe: `path, info, err`
- Si devolvés un error != nil, se detiene el walk

**Ejemplo:**
```
fastapi/
├── SKILL.md
└── scripts/
    └── test.py
```

Walk va a llamar tu callback con:
1. `path = "fastapi/SKILL.md"`
2. `path = "fastapi/scripts"`
3. `path = "fastapi/scripts/test.py"`

Filtramos con `!info.IsDir()` para quedarnos solo con archivos.

## Conceptos clave

- **Methods con receivers**: `func (r *Registry) Method()`
- **Constructor pattern**: Función `New()` que devuelve `*Struct`
- **String slicing**: `s[start:end]` para extraer substrings
- **filepath.Walk()**: Recorrer directorios recursivamente
- **Map como set**: `map[string]bool` para búsquedas O(1)
- **YAML frontmatter**: Metadata al inicio de archivos Markdown
- **Patrón "try but don't fail"**: Procesar opcionales sin fallar

---

**Tip para debuggear:**
```go
reg := registry.New("~/.agents/skills")
reg.Discover()
for _, skill := range reg.Skills {
    fmt.Printf("%s: %s\n", skill.Name, skill.Description)
}
```
