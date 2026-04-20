# Go — Guía de Conceptos Fundamentales

Esta es tu **hoja de referencia rápida** para los conceptos de Go que vas a encontrar en este proyecto.

## 📦 Packages

### ¿Qué es un package?

Es como un **módulo o namespace** — agrupa código relacionado.

```go
package main  // Package ejecutable (tiene func main)
package config // Package librería (usado por otros)
```

### Reglas:
- Cada archivo `.go` empieza declarando su package
- Todos los archivos en una carpeta deben tener el mismo package name
- El nombre del package suele ser el nombre de la carpeta

### Importar packages:
```go
import (
    "fmt"                    // Package de la stdlib
    "os"
    
    "github.com/user/repo/internal/config"  // Tu package
)
```

## 📊 Tipos de datos básicos

```go
// Strings
var name string = "sergio"
shortName := "sergio"  // Short declaration (tipo inferido)

// Números
var age int = 30
var price float64 = 19.99

// Booleanos
var isActive bool = true

// Arrays (tamaño fijo)
var arr [3]int = [3]int{1, 2, 3}

// Slices (tamaño dinámico — LO MÁS USADO)
var skills []string = []string{"go", "python"}
skills = append(skills, "rust")  // Agregar elemento
```

## 🔍 Slices (arrays dinámicos)

Los vas a usar TODO EL TIEMPO en Go.

```go
// Crear slice vacío
var nums []int

// Crear slice con valores
nums := []int{1, 2, 3}

// Crear slice con capacidad
nums := make([]int, 0, 10)  // len=0, cap=10

// Agregar elementos
nums = append(nums, 4)
nums = append(nums, 5, 6, 7)

// Acceder elementos
first := nums[0]
last := nums[len(nums)-1]

// Slicing (sub-array)
subset := nums[1:3]  // Desde índice 1 hasta 3 (exclusivo)
```

### Diferencias con arrays:
```go
// Array (tamaño fijo)
arr := [3]int{1, 2, 3}
// NO puedes cambiar el tamaño

// Slice (dinámico)
slice := []int{1, 2, 3}
slice = append(slice, 4)  // ✅ Crece dinámicamente
```

## 🗺️ Maps (diccionarios)

```go
// Crear map
ages := make(map[string]int)
ages["alice"] = 30
ages["bob"] = 25

// Crear con valores
ages := map[string]int{
    "alice": 30,
    "bob":   25,
}

// Acceder
age := ages["alice"]

// Verificar si existe
age, exists := ages["charlie"]
if exists {
    fmt.Println(age)
}

// Borrar key
delete(ages, "alice")

// Iterar
for name, age := range ages {
    fmt.Printf("%s: %d\n", name, age)
}
```

### Map como set:
```go
// Para verificar existencia rápida (O(1))
seen := make(map[string]bool)
seen["item1"] = true

if seen["item1"] {
    fmt.Println("Ya lo vimos")
}
```

## 🏗️ Structs (estructuras)

Como clases pero solo con **datos** (no métodos en la definición).

```go
// Definir struct
type Person struct {
    Name string
    Age  int
    Email string
}

// Crear instancia
p1 := Person{
    Name: "Sergio",
    Age: 30,
    Email: "sergio@example.com",
}

// Short form (orden importa)
p2 := Person{"Alice", 25, "alice@example.com"}

// Acceder campos
fmt.Println(p1.Name)
p1.Age = 31  // Modificar
```

### Structs anónimos:
```go
// Para uso temporal
config := struct {
    Host string
    Port int
}{
    Host: "localhost",
    Port: 8080,
}
```

## 👉 Punteros

Un puntero **guarda la dirección de memoria** de una variable.

```go
// Valor normal
x := 10

// Puntero a x
ptr := &x  // & = "dirección de"

// Acceder al valor
fmt.Println(*ptr)  // * = "dereference" → imprime 10

// Modificar a través del puntero
*ptr = 20
fmt.Println(x)  // x ahora es 20
```

### ¿Cuándo usar punteros?

#### 1. Para modificar el original:
```go
func increment(n *int) {
    *n++  // Modifica el original
}

x := 5
increment(&x)
fmt.Println(x)  // 6
```

#### 2. Para evitar copiar structs grandes:
```go
func process(cfg *Config) {
    // No copia todo el struct, solo pasa la referencia
}
```

#### 3. Como receivers de métodos:
```go
func (r *Registry) Discover() error {
    // Puede modificar r
    r.Skills = append(r.Skills, skill)
}
```

## 🔄 Funciones

```go
// Función básica
func add(a int, b int) int {
    return a + b
}

// Parámetros del mismo tipo
func add(a, b int) int {
    return a + b
}

// Múltiples retornos
func divide(a, b float64) (float64, error) {
    if b == 0 {
        return 0, fmt.Errorf("division by zero")
    }
    return a / b, nil
}

// Named returns (menos común)
func parse(s string) (result int, err error) {
    // result y err ya están declarados
    result = 10
    return  // Devuelve result y err
}

// Variadic (cantidad variable de argumentos)
func sum(nums ...int) int {
    total := 0
    for _, n := range nums {
        total += n
    }
    return total
}

sum(1, 2, 3)
sum(1, 2, 3, 4, 5)
```

## 🎯 Métodos (receivers)

Funciones asociadas a un tipo.

```go
type Rectangle struct {
    Width  float64
    Height float64
}

// Método con value receiver (no modifica)
func (r Rectangle) Area() float64 {
    return r.Width * r.Height
}

// Método con pointer receiver (puede modificar)
func (r *Rectangle) Scale(factor float64) {
    r.Width *= factor
    r.Height *= factor
}

// Uso
rect := Rectangle{Width: 10, Height: 5}
area := rect.Area()
rect.Scale(2)  // rect ahora es 20x10
```

### Value vs Pointer receiver:

| Receiver | Cuándo usar |
|----------|-------------|
| `(r Type)` | Método solo lee datos |
| `(r *Type)` | Método modifica datos O el struct es grande |

## ⚠️ Manejo de errores

Go NO tiene `try/catch`. Los errores son **valores** que devolvés.

```go
// Función que puede fallar
func readFile(path string) ([]byte, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("reading %s: %w", path, err)
    }
    return data, nil
}

// Caller maneja el error
data, err := readFile("config.yaml")
if err != nil {
    log.Fatal(err)  // Terminar programa
    // O manejar de otra forma
}
```

### Wrapping de errores:
```go
// %w = wraps (preserva el error original)
return fmt.Errorf("loading config: %w", err)

// Después puedes verificar el tipo
if errors.Is(err, os.ErrNotExist) {
    // El archivo no existe
}
```

### Error checking pattern:
```go
// Lo vas a ver TODO EL TIEMPO
if err := doSomething(); err != nil {
    return err
}
```

## 🔁 Loops

Go solo tiene **un tipo de loop**: `for`

```go
// For tradicional
for i := 0; i < 10; i++ {
    fmt.Println(i)
}

// While (no existe la keyword "while")
i := 0
for i < 10 {
    fmt.Println(i)
    i++
}

// Loop infinito
for {
    // break para salir
    if done {
        break
    }
}

// Range sobre slice
nums := []int{1, 2, 3}
for i, num := range nums {
    fmt.Printf("Index %d: %d\n", i, num)
}

// Range solo valores (ignora index)
for _, num := range nums {
    fmt.Println(num)
}

// Range sobre map
ages := map[string]int{"alice": 30, "bob": 25}
for name, age := range ages {
    fmt.Printf("%s: %d\n", name, age)
}
```

## 🔀 Condicionales

```go
// If básico
if x > 10 {
    fmt.Println("Grande")
}

// If con inicialización (común)
if err := doSomething(); err != nil {
    return err
}

// If-else
if x > 10 {
    fmt.Println("Grande")
} else if x > 5 {
    fmt.Println("Mediano")
} else {
    fmt.Println("Pequeño")
}

// Switch
switch day {
case "monday":
    fmt.Println("Lunes")
case "tuesday":
    fmt.Println("Martes")
default:
    fmt.Println("Otro día")
}

// Switch sin expresión (como if-else)
switch {
case x > 10:
    fmt.Println("Grande")
case x > 5:
    fmt.Println("Mediano")
default:
    fmt.Println("Pequeño")
}
```

## 🎫 Struct Tags

Metadatos para structs (usado por JSON, YAML, etc.)

```go
type User struct {
    Name  string `json:"name" yaml:"name"`
    Email string `json:"email" yaml:"email"`
    Age   int    `json:"age,omitempty" yaml:"age,omitempty"`
}
```

### Tags comunes:
- `json:"field_name"` — Para JSON
- `yaml:"field_name"` — Para YAML
- `,omitempty` — Omitir si está vacío
- `-` — Ignorar este campo

## 🗂️ Visibilidad (exportación)

Go usa capitalización para determinar visibilidad:

```go
// PÚBLICO (exportado) — accesible desde otros packages
type Config struct {
    Name string  // ✅ Público
}

func LoadConfig() *Config  // ✅ Público

// PRIVADO (no exportado) — solo en este package
type internalCache struct {
    data string  // ❌ Privado
}

func validate() error  // ❌ Privado
```

## 📁 Convenciones de estructura de proyecto

```
mi-proyecto/
├── go.mod              # Dependencias (como package.json)
├── go.sum              # Lock file de dependencias
├── README.md
├── cmd/                # Aplicaciones (main packages)
│   └── myapp/
│       └── main.go
├── internal/           # Código privado (no importable)
│   ├── config/
│   ├── database/
│   └── handlers/
└── pkg/                # Código público (importable por otros)
    └── utils/
```

## 🔧 Comandos útiles de Go

```bash
# Ejecutar sin compilar
go run main.go

# Compilar
go build -o myapp ./cmd/myapp

# Instalar dependencias
go mod tidy

# Agregar dependencia
go get github.com/user/package

# Ver dependencias
go mod graph

# Formatear código
go fmt ./...

# Tests
go test ./...

# Ver documentación
go doc fmt.Println
```

## 💡 Patrones comunes

### Constructor pattern:
```go
func New(config Config) *Service {
    return &Service{
        config: config,
    }
}
```

### Options pattern:
```go
type Option func(*Config)

func WithTimeout(d time.Duration) Option {
    return func(c *Config) {
        c.Timeout = d
    }
}

func NewClient(opts ...Option) *Client {
    cfg := &Config{}
    for _, opt := range opts {
        opt(cfg)
    }
    return &Client{config: cfg}
}
```

### Error wrapping:
```go
if err != nil {
    return fmt.Errorf("doing X: %w", err)
}
```

## 🚦 Principios de Go

1. **Simplicidad**: Si parece complicado, probablemente hay una forma más simple
2. **Explícito > Implícito**: Mejor ver el código que magia oculta
3. **Composición > Herencia**: Go no tiene herencia, usa interfaces y composición
4. **Errores como valores**: No excepciones, errores explícitos
5. **Formateo consistente**: `go fmt` formatea TODO igual

---

## 🎯 Siguiente paso

Lee los READMEs de las carpetas en este orden:
1. `internal/README.md` — Arquitectura general
2. `cmd/skillsync/README.md` — Entry point
3. `internal/config/README.md` — Manejo de config
4. Y así sucesivamente...

¡Dale que vas bien! 🚀
