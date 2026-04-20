# internal/tui

## ¿Qué hace este paquete?

Crea la **interfaz de usuario en terminal** (TUI = Terminal User Interface) usando la librería **Charm** (Bubbletea + Huh).

Es el **wizard interactivo** que guía al usuario a través de:
1. Elegir scope (global o project)
2. Elegir bundle o skills individuales
3. Seleccionar herramientas (Claude, Copilot, etc.)
4. Confirmar e instalar

## Librerías de Charm usadas

- **huh**: Componentes de formularios (Select, MultiSelect, Confirm)
- **lipgloss**: Estilos y colores para la terminal
- **bubbletea**: Framework de TUI (no usado directamente acá, pero es la base de huh)

## Conceptos de Go que vas a ver acá

### 1. Variables de package (estilos)
```go
var (
    titleStyle = lipgloss.NewStyle().
        Bold(true).
        Foreground(lipgloss.Color("#7C3AED"))
    
    successStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color("#10B981"))
)
```
- Variables que existen para todo el package
- Declara múltiples variables con `var (...)`
- Los estilos son como CSS pero para la terminal

### 2. Composición de structs
```go
type WizardResult struct {
    SelectedBundle string
    SelectedSkills []string
    SelectedTools  []string
    Scope          installer.Scope
    ProjectDir     string
}
```
- Un struct que **agrupa todos los resultados** del wizard
- Lo devolvés al final para que el caller sepa qué eligió el usuario

### 3. Fluent API / Builder pattern
```go
form := huh.NewForm(
    huh.NewGroup(
        huh.NewSelect[string]().
            Title("Scope").
            Options(...).
            Value(&scopeStr),
    ),
).WithTheme(huh.ThemeCatppuccin())
```
- Cada método devuelve el objeto mismo → puedes encadenar llamadas
- `.Value(&scopeStr)` → le pasás un **puntero** para que huh lo modifique
- El resultado se guarda directamente en tu variable

## Función principal: RunWizard()

```go
func RunWizard(cfg *config.Config, reg *registry.Registry, projectDir string) (*WizardResult, error)
```

### Flujo del wizard:

```
1. Imprimir título
         ↓
2. Scope: ¿Global o Project?
         ↓
3. Selección: ¿Bundle o skills individuales?
         ↓
4. Si bundle → elegir bundle
   Si individual → multi-select de skills
         ↓
5. Auto-detectar tecnologías (mostrar info)
         ↓
6. Multi-select de herramientas
         ↓
7. Mostrar resumen
         ↓
8. Confirmar
         ↓
9. Devolver WizardResult
```

### Paso 1: Scope (Global vs Project)

```go
var scopeStr string
scopeForm := huh.NewForm(
    huh.NewGroup(
        huh.NewSelect[string]().
            Title("Installation scope").
            Options(
                huh.NewOption("Global (home directory)", "global"),
                huh.NewOption("Project (current directory)", "project"),
            ).
            Value(&scopeStr),  // ← scopeStr se modifica cuando el usuario elige
    ),
).WithTheme(huh.ThemeCatppuccin())

if err := scopeForm.Run(); err != nil {
    return nil, err
}

// Convertir string a enum
if scopeStr == "global" {
    result.Scope = installer.ScopeGlobal
} else {
    result.Scope = installer.ScopeProject
}
```

**Conceptos clave:**
- `Value(&scopeStr)` → pasa un **puntero** para que huh escriba ahí
- `scopeForm.Run()` → **bloquea** hasta que el usuario elija
- Después del `Run()`, `scopeStr` tiene el valor elegido

### Paso 2: Bundle vs Individual

```go
var selectionMode string
if len(cfg.Bundles) > 0 {
    // Mostrar opción solo si hay bundles configurados
    modeForm := huh.NewForm(...)
    modeForm.Run()
} else {
    // Si no hay bundles, ir directo a individual
    selectionMode = "individual"
}
```

**Patrón: Skip step si no aplica**
- Si no hay bundles en el config, no tiene sentido preguntar
- Seteamos directamente `selectionMode = "individual"`

### Paso 3a: Elegir bundle (si modo = bundle)

```go
bundleOptions := make([]huh.Option[string], 0, len(cfg.Bundles))
for _, b := range cfg.Bundles {
    label := b.Name
    if b.Company != "" {
        label = fmt.Sprintf("%s (%s)", b.Name, b.Company)
    }
    bundleOptions = append(bundleOptions, huh.NewOption(label, b.Name))
}

bundleForm := huh.NewForm(
    huh.NewGroup(
        huh.NewSelect[string]().
            Title("Select bundle").
            Options(bundleOptions...).  // ← spread operator
            Value(&result.SelectedBundle),
    ),
)
bundleForm.Run()
```

**Conceptos:**
- Construyes un slice de opciones dinámicamente
- `Options(bundleOptions...)` → el `...` "desempaca" el slice
- Equivalente a `Options(bundleOptions[0], bundleOptions[1], ...)`

**Después de elegir el bundle, resolvemos las skills:**
```go
for _, b := range cfg.Bundles {
    if b.Name == result.SelectedBundle {
        for _, sr := range b.Skills {
            result.SelectedSkills = append(result.SelectedSkills, sr.Name)
        }
        break
    }
}
```

### Paso 3b: Pick individual skills (si modo = individual)

```go
skillOptions := make([]huh.Option[string], 0, len(reg.Skills))
for _, s := range reg.Skills {
    label := s.Name
    if s.Description != "" {
        label = fmt.Sprintf("%s — %s", s.Name, s.Description)
    }
    skillOptions = append(skillOptions, huh.NewOption(label, s.Name))
}

skillForm := huh.NewForm(
    huh.NewGroup(
        huh.NewMultiSelect[string]().  // ← MultiSelect permite elegir varios
            Title("Select skills to install").
            Options(skillOptions...).
            Value(&result.SelectedSkills),  // ← Se guarda un []string
    ),
)
skillForm.Run()
```

**MultiSelect vs Select:**
- `Select` → eliges UNO → devuelve `string`
- `MultiSelect` → eliges VARIOS → devuelve `[]string`

### Paso 4: Detectar tecnologías

```go
if projectDir != "" {
    techs := detector.Detect(projectDir)
    if len(techs) > 0 {
        fmt.Println(dimStyle.Render(
            fmt.Sprintf("  Detected tech: %s", strings.Join(techs, ", "))
        ))
    }
}
```

**Solo informativo:**
- Muestra las tecnologías detectadas
- No afecta el flujo (por ahora)
- En el futuro podría usarse para filtrar/recomendar skills

### Paso 5: Elegir herramientas

```go
// Pre-seleccionar herramientas que están habilitadas
var preSelected []string
for _, t := range cfg.Tools {
    if t.Enabled {
        preSelected = append(preSelected, t.Name)
    }
}
result.SelectedTools = preSelected

toolForm := huh.NewForm(
    huh.NewGroup(
        huh.NewMultiSelect[string]().
            Title("Target agentic tools").
            Options(toolOptions...).
            Value(&result.SelectedTools),  // Ya tiene valores pre-seleccionados
    ),
)
toolForm.Run()
```

**Pre-selección:**
- Inicializas `result.SelectedTools` con valores por defecto
- El usuario puede cambiarlos en el MultiSelect
- Mejora la UX (menos clicks)

### Paso 6: Mostrar resumen

```go
fmt.Println(headerStyle.Render("\n📋 Installation Summary"))
fmt.Printf("  Skills:  %s\n", strings.Join(result.SelectedSkills, ", "))
fmt.Printf("  Tools:   %s\n", strings.Join(result.SelectedTools, ", "))
fmt.Printf("  Scope:   %s\n", scopeStr)
```

**strings.Join():**
```go
skills := []string{"fastapi", "sdd-init"}
s := strings.Join(skills, ", ")
// s = "fastapi, sdd-init"
```

### Paso 7: Confirmar

```go
var confirm bool
confirmForm := huh.NewForm(
    huh.NewGroup(
        huh.NewConfirm().
            Title("Proceed with installation?").
            Value(&confirm),
    ),
)
confirmForm.Run()

if !confirm {
    return nil, fmt.Errorf("installation cancelled")
}
```

- `Confirm` → pregunta sí/no
- Si el usuario dice no, devolvemos un error
- El caller puede manejar la cancelación

## Función: PrintResults()

```go
func PrintResults(results []installer.Result) {
    created, existed, errors := 0, 0, 0
    
    for _, r := range results {
        switch {
        case r.Error != nil:
            fmt.Printf("  %s %s → %s: %s\n",
                errorStyle.Render("✗"), r.Skill, r.Tool, r.Error)
            errors++
        case r.Existed:
            fmt.Printf("  %s %s → %s %s\n",
                dimStyle.Render("○"), r.Skill, r.Tool,
                dimStyle.Render("(already exists)"))
            existed++
        case r.Created:
            fmt.Printf("  %s %s → %s\n",
                successStyle.Render("✓"), r.Skill, r.Tool)
            created++
        }
    }
    
    fmt.Printf("\n  %s  %s  %s\n",
        successStyle.Render(fmt.Sprintf("%d created", created)),
        dimStyle.Render(fmt.Sprintf("%d existing", existed)),
        errorStyle.Render(fmt.Sprintf("%d errors", errors)))
}
```

### Switch sin expresión
```go
switch {
case r.Error != nil:
    // caso 1
case r.Existed:
    // caso 2
case r.Created:
    // caso 3
}
```
- Cuando no ponés expresión después del `switch`, evalúa cada `case` como un booleano
- Es equivalente a `if/else if/else` pero más limpio

### Colores y símbolos
- ✓ (check) en verde → éxito
- ○ (círculo) en gris → ya existía
- ✗ (X) en rojo → error

## Función: DetectInstalledTools()

```go
func DetectInstalledTools(tools []config.Tool) []config.Tool {
    updated := make([]config.Tool, len(tools))
    copy(updated, tools)  // Copia el slice original
    
    for i := range updated {
        globalPath := config.ExpandPath(updated[i].GlobalPath)
        parentDir := strings.TrimSuffix(globalPath, "/skills")
        
        if _, err := os.Stat(parentDir); err == nil {
            updated[i].Enabled = true
        }
    }
    
    return updated
}
```

### ¿Qué hace?

Detecta qué herramientas están instaladas buscando sus directorios:
- `~/.claude/skills` → busca si existe `~/.claude/`
- `~/.copilot/skills` → busca si existe `~/.copilot/`

**Patrón: Copy-on-write**
```go
updated := make([]config.Tool, len(tools))
copy(updated, tools)
```
- Crea una **copia** del slice original
- No modifica el original (inmutabilidad)
- Devuelve la versión modificada

### strings.TrimSuffix()
```go
path := "~/.claude/skills"
parent := strings.TrimSuffix(path, "/skills")
// parent = "~/.claude"
```

### os.Stat() para verificar existencia
```go
if _, err := os.Stat(parentDir); err == nil {
    // El directorio existe
}
```
- Si `err == nil` → no hay error → el path existe
- Ignoramos el primer retorno (info del archivo) con `_`

## lipgloss — Estilos en terminal

```go
titleStyle := lipgloss.NewStyle().
    Bold(true).
    Foreground(lipgloss.Color("#7C3AED")).
    MarginBottom(1)

fmt.Println(titleStyle.Render("⚡ skillsync"))
```

**API fluida:**
- `Bold(true)` → negrita
- `Foreground(color)` → color del texto
- `MarginBottom(1)` → espacio debajo
- `Render(text)` → aplica el estilo al texto

Es como CSS pero en Go para terminales.

## Conceptos clave

- **TUI con huh**: Formularios interactivos en terminal
- **lipgloss**: Estilos y colores
- **Value() con puntero**: Para que la librería escriba el resultado
- **form.Run()**: Bloquea hasta que el usuario complete el form
- **MultiSelect vs Select**: Múltiples opciones vs una sola
- **Switch sin expresión**: Para múltiples condiciones booleanas
- **Copy-on-write**: `copy()` para no mutar el original
- **Variadic functions**: `Options(opts...)` desempaca un slice

---

**Tip para testear el wizard:**
```bash
go run cmd/skillsync/main.go
# Navegá con flechas, Space para seleccionar, Enter para continuar
```
