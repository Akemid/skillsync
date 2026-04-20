# 📚 Índice de Documentación — skillsync

Esta es tu **guía completa** para entender el proyecto skillsync y aprender Go en el proceso.

## 🎯 Empezá por acá

Si estás aprendiendo Go o querés refrescar conceptos:

1. **[Go Fundamentals](docs/GO_FUNDAMENTALS.md)** ⭐
   - Conceptos básicos de Go explicados con ejemplos
   - Slices, maps, structs, punteros, errores, etc.
   - Referencia rápida que vas a consultar todo el tiempo

2. **[Arquitectura del Proyecto](internal/README.md)** ⭐
   - Cómo se conectan todas las partes
   - Flujo completo de una instalación
   - Separación de responsabilidades
   - Patrones de diseño utilizados

## 📦 Documentación por Package

Lee en este orden para entender el flujo completo:

### 1. Entry Point
- **[cmd/skillsync](cmd/skillsync/README.md)**
  - El punto de entrada de la aplicación
  - Manejo de argumentos de línea de comandos
  - Coordinación de todos los componentes
  - Conceptos: `package main`, `func main()`, error handling

### 2. Configuración
- **[internal/config](internal/config/README.md)**
  - Lectura y escritura de archivos YAML
  - Estructuras de datos (structs)
  - Valores por defecto
  - Conceptos: structs, tags, pointers, YAML marshaling

### 3. Registro de Skills
- **[internal/registry](internal/registry/README.md)**
  - Escaneo del directorio de skills
  - Parseo de frontmatter YAML
  - Búsqueda de skills
  - Conceptos: methods, receivers, string slicing, filepath.Walk

### 4. Detección de Tecnologías
- **[internal/detector](internal/detector/README.md)**
  - Detección automática de frameworks
  - Búsqueda de archivos con glob patterns
  - Análisis de contenido de archivos
  - Conceptos: maps, loops, file operations

### 5. Instalación
- **[internal/installer](internal/installer/README.md)**
  - Creación de enlaces simbólicos (symlinks)
  - Instalación y desinstalación
  - Manejo de resultados
  - Conceptos: enums con iota, bitwise operations, filepath operations

### 6. Interfaz de Usuario
- **[internal/tui](internal/tui/README.md)**
  - Wizard interactivo en terminal
  - Formularios con la librería Charm
  - Estilos y colores
  - Conceptos: fluent API, forms, multiselect, styling

## 🗺️ Mapa Mental del Código

```
Usuario ejecuta: skillsync
         │
         ▼
    cmd/skillsync/main.go (COORDINADOR)
         │
         ├─→ internal/config
         │    └─ Lee skillsync.yaml
         │    └─ Devuelve Config struct
         │
         ├─→ internal/registry
         │    └─ Escanea ~/.agents/skills/
         │    └─ Devuelve lista de Skills
         │
         ├─→ internal/detector
         │    └─ Detecta tecnologías del proyecto
         │    └─ Devuelve lista de strings (techs)
         │
         ├─→ internal/tui
         │    └─ Muestra wizard interactivo
         │    └─ Usuario elige skills y tools
         │    └─ Devuelve WizardResult
         │
         └─→ internal/installer
              └─ Crea symlinks
              └─ Devuelve resultados (éxitos/errores)
```

## 🎓 Ruta de Aprendizaje Sugerida

### Nivel 1: Fundamentos (1-2 horas)
1. Lee [Go Fundamentals](docs/GO_FUNDAMENTALS.md)
2. Ejecutá el programa: `go run cmd/skillsync/main.go`
3. Lee [cmd/skillsync/README.md](cmd/skillsync/README.md)

### Nivel 2: Arquitectura (2-3 horas)
4. Lee [internal/README.md](internal/README.md)
5. Lee [internal/config/README.md](internal/config/README.md)
6. Modificá algo simple: cambia un valor por defecto en config

### Nivel 3: Componentes (4-5 horas)
7. Lee [internal/registry/README.md](internal/registry/README.md)
8. Lee [internal/detector/README.md](internal/detector/README.md)
9. Agrega detección de un nuevo lenguaje en detector.go

### Nivel 4: Features Avanzados (3-4 horas)
10. Lee [internal/installer/README.md](internal/installer/README.md)
11. Lee [internal/tui/README.md](internal/tui/README.md)
12. Cambia colores o mensajes en el TUI

### Nivel 5: Contribuye
13. Agrega tests para algún package
14. Implementa una nueva feature
15. Documenta lo que aprendiste

## 🔍 Buscar Conceptos Específicos

### Conceptos de Go:

| Concepto | Dónde leerlo |
|----------|-------------|
| Slices y arrays | [Go Fundamentals](docs/GO_FUNDAMENTALS.md) |
| Maps como sets | [detector/README.md](internal/detector/README.md) |
| Structs y tags | [config/README.md](internal/config/README.md) |
| Punteros | [Go Fundamentals](docs/GO_FUNDAMENTALS.md) |
| Métodos con receivers | [registry/README.md](internal/registry/README.md) |
| Manejo de errores | [Go Fundamentals](docs/GO_FUNDAMENTALS.md) |
| Enums con iota | [installer/README.md](internal/installer/README.md) |
| String slicing | [registry/README.md](internal/registry/README.md) |
| File operations | [detector/README.md](internal/detector/README.md) |
| Symlinks | [installer/README.md](internal/installer/README.md) |

### Features del proyecto:

| Feature | Dónde está |
|---------|-----------|
| Parseo de YAML | [config/README.md](internal/config/README.md) |
| Detección de tech | [detector/README.md](internal/detector/README.md) |
| Búsqueda de archivos | [registry/README.md](internal/registry/README.md) |
| Creación de symlinks | [installer/README.md](internal/installer/README.md) |
| TUI interactivo | [tui/README.md](internal/tui/README.md) |
| Frontmatter parsing | [registry/README.md](internal/registry/README.md) |

## 💡 Tips para Aprender

### 1. No leas todo de una vez
- Elegí un package que te interese
- Lee su README completo
- Después lee el código fuente
- Hacé cambios pequeños para entender

### 2. Experimentá
```bash
# Modificá algo y mirá qué pasa
go run cmd/skillsync/main.go

# Agregá prints para debuggear
fmt.Printf("Debug: %+v\n", variable)

# Usá go fmt para formatear
go fmt ./...
```

### 3. Consultá la documentación oficial
- [Tour of Go](https://tour.golang.org/) — Interactive tutorial
- [Effective Go](https://go.dev/doc/effective_go) — Best practices
- [Go by Example](https://gobyexample.com/) — Practical examples

### 4. Seguí los patrones del proyecto
- Constructor pattern: `func New() *Type`
- Error wrapping: `fmt.Errorf("context: %w", err)`
- Pointer receivers: `func (r *Registry) Method()`

## 🤝 Contribuir

Si agregás nuevas features o modificás código existente:

1. **Documentá tus cambios** — actualiza el README del package
2. **Mantené el estilo** — usa `go fmt` siempre
3. **Explica conceptos** — la documentación es para aprender
4. **Agrega ejemplos** — el código de ejemplo ayuda mucho

## 📞 Recursos Adicionales

- [README principal](README.md) — Info general del proyecto
- [skillsync.example.yaml](skillsync.example.yaml) — Ejemplo de configuración
- [Agent Skills](https://agentskills.io/) — El estándar de skills que usamos

---

**¿Por dónde empezar?** → [Go Fundamentals](docs/GO_FUNDAMENTALS.md) ⭐

**¿Querés entender el flujo?** → [Arquitectura](internal/README.md) ⭐

**¿Querés modificar algo?** → Elegí un package y empezá a experimentar 🚀
