# cmd/skillsync

## ¿Qué es esta carpeta?

Esta es la **carpeta de comandos** del proyecto. En Go, por convención, ponés el código ejecutable (el que tiene el `func main()`) en la carpeta `cmd/`.

## Conceptos de Go que vas a ver acá

### 1. Package main
```go
package main
```
- Es el ÚNICO package que puede tener una función `main()`
- Cuando compilás con `go build`, Go busca el `package main` y lo convierte en ejecutable
- Cualquier otro package (como `config`, `registry`, etc.) es una librería

### 2. func main()
```go
func main() {
    if err := run(); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}
```
- Es el **punto de entrada** de tu programa
- Se mantiene simple: solo llama a `run()` y maneja errores globales
- Si hay un error, imprime a `stderr` y sale con código 1 (indica error al sistema operativo)

### 3. Patrón de manejo de errores
```go
if err := run(); err != nil {
    // manejar el error
}
```
- En Go NO hay `try/catch`. Los errores son valores que devolvés explícitamente
- La convención es: la última cosa que devolvés es un `error`
- Si `err != nil`, algo salió mal

## ¿Qué hace main.go?

Este archivo es el **coordinador principal** de `skillsync`. Hace lo siguiente:

### 1. Maneja los argumentos de línea de comandos
```go
if len(os.Args) > 1 {
    switch os.Args[1] {
    case "list":
        return cmdList(reg)
    case "status":
        return cmdStatus(cfg, reg)
    // etc...
    }
}
```
- `os.Args` es un slice (array dinámico) con los argumentos
- `os.Args[0]` es el nombre del programa
- `os.Args[1]` es el primer argumento que pasó el usuario

### 2. Carga la configuración
```go
cfg, err := config.Load(configPath)
if err != nil {
    // Si no existe, usa defaults
    if errors.Is(err, os.ErrNotExist) {
        cfg = &config.Config{...}
    }
}
```
- Intenta cargar el config desde el archivo YAML
- Si no existe, usa valores por defecto
- `errors.Is()` es para comparar tipos de errores específicos

### 3. Inicializa el Registry
```go
reg := registry.New(cfg.RegistryPath)
if err := reg.Discover(); err != nil {
    return fmt.Errorf("scanning skill registry: %w", err)
}
```
- Crea un nuevo registro de skills
- `Discover()` escanea el directorio para encontrar todas las skills
- `%w` en `fmt.Errorf()` envuelve el error original (wrapping) — útil para debugging

### 4. Delega a subcomandos o al wizard
- Si hay un comando específico (`list`, `status`, etc.), ejecuta esa función
- Si no hay comandos, lanza el wizard interactivo (TUI)

## Flujo del programa

```
Usuario ejecuta: skillsync list
         ↓
    func main()
         ↓
    func run()
         ↓
    Carga config
         ↓
    Inicializa registry
         ↓
    Detecta comando "list"
         ↓
    Ejecuta cmdList()
         ↓
    Imprime lista de skills
```

## ¿Por qué separar main() y run()?

Es un **patrón común en Go**:
- `main()` se mantiene simple y solo maneja el exit code
- `run()` contiene toda la lógica y devuelve errores
- Esto hace el código más testeable (podés testear `run()` sin ejecutar `main()`)

## Conceptos clave que usamos

- **os.Args**: Argumentos de línea de comandos
- **os.Getwd()**: Get Working Directory (directorio actual)
- **os.Exit(1)**: Salir del programa con código de error
- **fmt.Errorf()**: Crear un error con formato
- **switch/case**: Como en otros lenguajes, pero sin `break` (no hace fallthrough automático)

---

**Tip**: Si querés ver qué argumentos recibe tu programa, agregá esto temporalmente:
```go
fmt.Println("Args:", os.Args)
```
