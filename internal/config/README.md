# internal/config

## ¿Qué hace este paquete?

Maneja toda la **configuración** de `skillsync`:
- Lee y escribe archivos YAML
- Define las estructuras de datos (structs) para la config
- Expande paths con `~` (home directory)
- Provee valores por defecto

## Conceptos de Go que vas a ver acá

### 1. Structs (Estructuras)
```go
type Tool struct {
    Name       string `yaml:"name"`
    GlobalPath string `yaml:"global_path"`
    Enabled    bool   `yaml:"enabled"`
}
```
- Los **structs** son como clases sin métodos (datos agrupados)
- Los **tags** (`` `yaml:"name"` ``) le dicen al parser de YAML cómo mapear los campos
- Los campos que empiezan con **mayúscula** son públicos (exportados)
- Los que empiezan con minúscula son privados (solo visibles en este package)

### 2. Punteros
```go
func Load(path string) (*Config, error)
```
- `*Config` significa "un puntero a Config"
- Los punteros te dejan modificar el original (no una copia)
- En Go, cuando devolvés structs grandes, usás punteros para evitar copiar todo en memoria

### 3. Tags de struct
```go
type Tool struct {
    Name string `yaml:"name"`
    Tags []string `yaml:"tags,omitempty"`
}
```
- `` `yaml:"name"` `` → mapea este campo al key "name" en el YAML
- `,omitempty` → si el campo está vacío, no lo incluye en el YAML al guardarlo

### 4. Slices
```go
type Bundle struct {
    Skills []SkillRef `yaml:"skills"`
}
```
- `[]SkillRef` es un **slice** (array dinámico)
- No necesitás especificar el tamaño
- Crecen automáticamente cuando agregás elementos

## Estructuras de datos principales

### Config (configuración principal)
```go
type Config struct {
    RegistryPath string   `yaml:"registry_path"` 
    Bundles      []Bundle `yaml:"bundles"`
    Tools        []Tool   `yaml:"tools"`
}
```
- **RegistryPath**: Dónde están las skills centralizadas (`~/.agents/skills/`)
- **Bundles**: Grupos predefinidos de skills (ej: "personal", "work")
- **Tools**: Lista de herramientas soportadas (Claude, Copilot, etc.)

### Tool (herramienta soportada)
```go
type Tool struct {
    Name       string `yaml:"name"`       // ej: "claude"
    GlobalPath string `yaml:"global_path"` // ej: "~/.claude/skills"
    LocalPath  string `yaml:"local_path"`  // ej: ".claude/skills"
    Enabled    bool   `yaml:"enabled"`     // ¿Está instalada?
}
```

### Bundle (grupo de skills)
```go
type Bundle struct {
    Name        string     `yaml:"name"`
    Description string     `yaml:"description,omitempty"`
    Skills      []SkillRef `yaml:"skills"`
}
```

## Funciones importantes

### Load() - Cargar configuración
```go
func Load(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("reading config: %w", err)
    }
    var cfg Config
    if err := yaml.Unmarshal(data, &cfg); err != nil {
        return nil, fmt.Errorf("parsing config: %w", err)
    }
    return &cfg, nil
}
```

**Flujo:**
1. Lee el archivo completo con `os.ReadFile()` → devuelve `[]byte` (array de bytes)
2. Parsea el YAML con `yaml.Unmarshal()` → convierte bytes a struct
3. Si algo falla, devuelve `nil` y el error
4. Si todo bien, devuelve un puntero al config

### Save() - Guardar configuración
```go
func Save(cfg *Config, path string) error {
    data, err := yaml.Marshal(cfg)
    if err != nil {
        return fmt.Errorf("marshaling config: %w", err)
    }
    return os.WriteFile(path, data, 0644)
}
```

**Flujo:**
1. Convierte el struct a YAML con `yaml.Marshal()` → devuelve `[]byte`
2. Escribe los bytes al archivo con `os.WriteFile()`
3. `0644` son los permisos Unix (owner: read/write, group: read, others: read)

### ExpandPath() - Expandir `~` a home directory
```go
func ExpandPath(p string) string {
    if len(p) > 1 && p[:2] == "~/" {
        home, err := os.UserHomeDir()
        if err != nil {
            return p // Si falla, devuelve el original
        }
        return filepath.Join(home, p[2:])
    }
    return p
}
```

**Qué hace:**
- Convierte `~/.agents/skills` → `/Users/tu-usuario/.agents/skills`
- `p[:2]` → slice de string, primeros 2 caracteres
- `p[2:]` → desde el carácter 3 hasta el final
- `filepath.Join()` → une paths de forma portable (funciona en Windows, Linux, macOS)

## Patrón: Valores por defecto

```go
func DefaultTools() []Tool {
    return []Tool{
        {Name: "claude", GlobalPath: "~/.claude/skills", Enabled: true},
        {Name: "copilot", GlobalPath: "~/.copilot/skills", Enabled: true},
        // ...
    }
}
```

- Si el usuario no tiene un config, usamos estos valores
- Los devuelve como un slice literal (creado directamente en el return)

## Conceptos clave

- **Struct tags**: Mapean campos a formato externo (JSON, YAML, etc.)
- **Unmarshal/Marshal**: Convertir entre bytes y structs
- **os.ReadFile/WriteFile**: Leer/escribir archivos completos de una vez
- **filepath.Join**: Une paths de forma segura y portable
- **Error wrapping** (`%w`): Mantiene la cadena de errores para debugging

---

**Tip**: Si querés ver cómo se parsea un YAML, podés hacer:
```go
cfg, _ := config.Load("skillsync.yaml")
fmt.Printf("%+v\n", cfg) // Imprime el struct con nombres de campos
```
