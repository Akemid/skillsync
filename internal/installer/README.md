# internal/installer

## ¿Qué hace este paquete?

**Instala y desinstala skills** creando **symlinks** (enlaces simbólicos) desde los directorios de las herramientas hacia el registry central.

### ¿Qué es un symlink?

Un enlace simbólico es como un "atajo" o "acceso directo":
- No copia archivos, solo crea una referencia
- Si actualizás el original, el symlink refleja el cambio automáticamente
- Ahorra espacio en disco

```
~/.agents/skills/fastapi/  ← Skill original (1 copia)
         ↑
         │ (symlink)
         │
~/.claude/skills/fastapi → apunta al original
~/.copilot/skills/fastapi → apunta al original
```

## Conceptos de Go que vas a ver acá

### 1. Enums con iota
```go
type Scope int

const (
    ScopeGlobal  Scope = iota // 0
    ScopeProject              // 1
)
```
- **iota** es un contador automático que empieza en 0
- Se incrementa por cada constante en el bloque
- Usamos un custom type (`Scope`) para type safety

### 2. Switch sin expresión
```go
switch scope {
case ScopeGlobal:
    basePath = config.ExpandPath(tool.GlobalPath)
case ScopeProject:
    basePath = filepath.Join(projectDir, tool.LocalPath)
}
```
- No necesitás `break` en Go (no hace fallthrough automático)
- Es más limpio que múltiples `if/else`

### 3. Acumulación de resultados
```go
var results []Result
for _, tool := range tools {
    for _, skill := range skills {
        res := Result{...}
        // ... lógica ...
        results = append(results, res)
    }
}
return results
```
- Crea un slice vacío
- Agrega resultados con `append()` mientras procesa
- Devuelve todo al final

## Struct: Result

```go
type Result struct {
    Tool    string
    Skill   string
    Target  string  // Path donde se creó el symlink
    Created bool    // ¿Se creó nuevo?
    Existed bool    // ¿Ya existía?
    Error   error   // Si hubo error
}
```

Este struct te deja **reportar qué pasó** con cada intento de instalación:
- ✅ Creado → `Created = true`
- ○ Ya existe → `Existed = true`
- ✗ Error → `Error != nil`

## Función principal: Install()

```go
func Install(skills []registry.Skill, tools []config.Tool, scope Scope, projectDir string) []Result
```

### Parámetros:
- `skills`: Las skills a instalar
- `tools`: Las herramientas donde instalarlas
- `scope`: Global (~/.claude/) o Project (.claude/)
- `projectDir`: Directorio del proyecto actual

### ¿Qué hace?

```go
for _, tool := range tools {
    // 1. Determinar base path (global o project)
    var basePath string
    switch scope {
    case ScopeGlobal:
        basePath = config.ExpandPath(tool.GlobalPath)
    case ScopeProject:
        basePath = filepath.Join(projectDir, tool.LocalPath)
    }
    
    for _, skill := range skills {
        target := filepath.Join(basePath, skill.Name)
        
        // 2. Verificar si ya existe
        if info, err := os.Lstat(target); err == nil {
            // Ya existe, skip
            continue
        }
        
        // 3. Crear directorio padre si no existe
        os.MkdirAll(basePath, 0755)
        
        // 4. Crear symlink relativo
        relPath, _ := filepath.Rel(basePath, skill.Path)
        os.Symlink(relPath, target)
    }
}
```

### Detalles importantes:

#### os.Lstat() vs os.Stat()
```go
info, err := os.Lstat(target)
```
- `Lstat()` → info del **symlink mismo** (no sigue el link)
- `Stat()` → info del archivo **apuntado** por el symlink
- Usamos `Lstat()` para detectar si el symlink existe, aunque apunte a algo que no existe

#### Verificar si es un symlink
```go
if info.Mode()&os.ModeSymlink != 0 {
    // Es un symlink
}
```
- `info.Mode()` devuelve un bitmask con flags del archivo
- `os.ModeSymlink` es un flag específico
- `&` es operación bitwise AND para verificar si el flag está activo

#### Crear symlinks relativos (no absolutos)
```go
relPath, err := filepath.Rel(basePath, skill.Path)
os.Symlink(relPath, target)
```

**¿Por qué relativo?**
- Si movés tu home directory, los symlinks absolutos se rompen
- Los relativos (`../../.agents/skills/fastapi`) siguen funcionando

Ejemplo:
```
basePath: /Users/sergio/.claude/skills
skill.Path: /Users/sergio/.agents/skills/fastapi
relPath: ../../.agents/skills/fastapi
```

#### Permisos de directorio: 0755
```go
os.MkdirAll(basePath, 0755)
```
- `0755` en octal = `rwxr-xr-x` en Unix
- Owner: read, write, execute
- Group: read, execute
- Others: read, execute

## Función: Uninstall()

```go
func Uninstall(skillNames []string, tools []config.Tool, scope Scope, projectDir string) []Result
```

Similar a `Install()`, pero:

### 1. Solo borra symlinks (seguridad)
```go
info, err := os.Lstat(target)
if info.Mode()&os.ModeSymlink == 0 {
    res.Error = fmt.Errorf("not a symlink, skipping for safety")
    continue
}
```
- **Nunca borra directorios reales**
- Solo remueve si es un symlink
- Esto previene borrados accidentales

### 2. Usa os.Remove() para borrar
```go
if err := os.Remove(target); err != nil {
    res.Error = err
} else {
    res.Created = true // reutilizamos el campo para indicar éxito
}
```

## Patrón: Acumular resultados vs. Fallar rápido

Este código usa **acumular resultados**:
```go
for _, item := range items {
    res := processItem(item)
    results = append(results, res)
}
return results // Devuelve TODO, exitoso o fallido
```

**Alternativa "fail fast":**
```go
for _, item := range items {
    if err := processItem(item); err != nil {
        return err // Sale en el primer error
    }
}
```

¿Cuándo usar cada uno?
- **Acumular**: Cuando querés procesar todo y reportar todos los resultados (como acá)
- **Fail fast**: Cuando un error hace imposible continuar

## Conceptos clave

- **os.Symlink()**: Crea enlaces simbólicos
- **os.Lstat()**: Info del symlink (no sigue el link)
- **filepath.Rel()**: Calcula path relativo entre dos paths
- **os.MkdirAll()**: Crea directorio y todos sus padres (como `mkdir -p`)
- **Bitwise operations**: `info.Mode() & os.ModeSymlink` para verificar flags
- **Type safety con custom types**: `type Scope int` mejor que solo `int`

---

**Tip para debuggear symlinks:**
```bash
ls -la ~/.claude/skills/
# Vas a ver algo como:
# fastapi -> ../../.agents/skills/fastapi
```
