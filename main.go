package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// ─── Data Types ───────────────────────────────────────────────────────────────

type Question struct {
	ID          int      `json:"id"`
	Title       string   `json:"title"`
	Difficulty  string   `json:"difficulty"`
	Category    string   `json:"category"`
	Points      int      `json:"points"`
	TimeBonus   int      `json:"timeBonus"`
	Description string   `json:"description"`
	Template    string   `json:"template"`
	Hints       []string `json:"hints"`
}

type Player struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Points       int           `json:"points"`
	Level        int           `json:"level"`
	Streak       int           `json:"streak"`
	SolvedIDs    []int         `json:"solvedIds"`
	Achievements []Achievement `json:"achievements"`
	LastSolved   time.Time     `json:"lastSolved"`
}

type Achievement struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
}

type RunRequest struct {
	Code       string `json:"code"`
	QuestionID int    `json:"questionId"`
	PlayerID   string `json:"playerId"`
	StartTime  int64  `json:"startTime"`
}

type RunResponse struct {
	Success   bool          `json:"success"`
	Output    string        `json:"output"`
	Error     string        `json:"error"`
	Points    int           `json:"points"`
	TimeBonus int           `json:"timeBonus"`
	Unlocked  []Achievement `json:"unlocked"`
}

type LintRequest struct {
	Code string `json:"code"`
}

type LintResponse struct {
	Errors []string `json:"errors"`
}

type LeaderboardEntry struct {
	Rank   int    `json:"rank"`
	ID     string `json:"id"`
	Name   string `json:"name"`
	Points int    `json:"points"`
	Level  int    `json:"level"`
	Streak int    `json:"streak"`
}

// ─── In-Memory Store ──────────────────────────────────────────────────────────

var (
	mu       sync.RWMutex
	players  = map[string]*Player{}
	dailyQID = 1 // Could be randomized daily
)

// ─── Questions ────────────────────────────────────────────────────────────────

var questions = []Question{
	{ID: 1, Title: "Hello, World!", Difficulty: "easy", Category: "basics", Points: 50, TimeBonus: 30, Description: "Write a Go program that prints `Hello, World!` to the console.", Template: "package main\n\nimport \"fmt\"\n\nfunc main() {\n\t// Your code here\n}", Hints: []string{"Use fmt.Println()", "Don't forget the package declaration"}},
	{ID: 2, Title: "Sum Two Numbers", Difficulty: "easy", Category: "basics", Points: 60, TimeBonus: 45, Description: "Write a function `add(a, b int) int` that returns the sum of two integers. Then print `add(3, 7)` in main.", Template: "package main\n\nimport \"fmt\"\n\nfunc add(a, b int) int {\n\t// Your code here\n}\n\nfunc main() {\n\tfmt.Println(add(3, 7))\n}", Hints: []string{"Use the + operator", "Return the result"}},
	{ID: 3, Title: "FizzBuzz", Difficulty: "easy", Category: "loops", Points: 70, TimeBonus: 60, Description: "Print numbers 1 to 20. For multiples of 3 print `Fizz`, multiples of 5 print `Buzz`, multiples of both print `FizzBuzz`.", Template: "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfor i := 1; i <= 20; i++ {\n\t\t// Your code here\n\t}\n}", Hints: []string{"Use the % (modulo) operator", "Check FizzBuzz before Fizz and Buzz"}},
	{ID: 4, Title: "Reverse a String", Difficulty: "easy", Category: "strings", Points: 80, TimeBonus: 60, Description: "Write a function `reverse(s string) string` that reverses a string. Print the reverse of `golang`.", Template: "package main\n\nimport \"fmt\"\n\nfunc reverse(s string) string {\n\t// Your code here\n}\n\nfunc main() {\n\tfmt.Println(reverse(\"golang\"))\n}", Hints: []string{"Convert string to rune slice", "Iterate backwards"}},
	{ID: 5, Title: "Fibonacci", Difficulty: "easy", Category: "recursion", Points: 90, TimeBonus: 90, Description: "Write a recursive function `fib(n int) int` that returns the nth Fibonacci number. Print fib(10).", Template: "package main\n\nimport \"fmt\"\n\nfunc fib(n int) int {\n\t// Your code here\n}\n\nfunc main() {\n\tfmt.Println(fib(10))\n}", Hints: []string{"Base cases: fib(0)=0, fib(1)=1", "fib(n) = fib(n-1) + fib(n-2)"}},
	{ID: 6, Title: "Count Vowels", Difficulty: "easy", Category: "strings", Points: 70, TimeBonus: 45, Description: "Write `countVowels(s string) int` that counts vowels (a,e,i,o,u) in a string. Print countVowels(\"hello world\").", Template: "package main\n\nimport \"fmt\"\n\nfunc countVowels(s string) int {\n\t// Your code here\n}\n\nfunc main() {\n\tfmt.Println(countVowels(\"hello world\"))\n}", Hints: []string{"Loop over characters", "Check if char is in \"aeiou\""}},
	{ID: 7, Title: "Is Palindrome", Difficulty: "easy", Category: "strings", Points: 80, TimeBonus: 60, Description: "Write `isPalindrome(s string) bool`. Print results for \"racecar\" and \"golang\".", Template: "package main\n\nimport \"fmt\"\n\nfunc isPalindrome(s string) bool {\n\t// Your code here\n}\n\nfunc main() {\n\tfmt.Println(isPalindrome(\"racecar\"))\n\tfmt.Println(isPalindrome(\"golang\"))\n}", Hints: []string{"Compare string to its reverse"}},
	{ID: 8, Title: "Factorial", Difficulty: "easy", Category: "recursion", Points: 70, TimeBonus: 45, Description: "Write `factorial(n int) int` using recursion. Print factorial(5).", Template: "package main\n\nimport \"fmt\"\n\nfunc factorial(n int) int {\n\t// Your code here\n}\n\nfunc main() {\n\tfmt.Println(factorial(5))\n}", Hints: []string{"Base case: n <= 1 returns 1"}},
	{ID: 9, Title: "Max in Slice", Difficulty: "easy", Category: "slices", Points: 75, TimeBonus: 60, Description: "Write `maxSlice(nums []int) int` that returns the maximum value. Print maxSlice([]int{3,1,4,1,5,9,2,6}).", Template: "package main\n\nimport \"fmt\"\n\nfunc maxSlice(nums []int) int {\n\t// Your code here\n}\n\nfunc main() {\n\tfmt.Println(maxSlice([]int{3, 1, 4, 1, 5, 9, 2, 6}))\n}", Hints: []string{"Start with first element as max", "Loop and compare"}},
	{ID: 10, Title: "Word Count Map", Difficulty: "easy", Category: "maps", Points: 85, TimeBonus: 75, Description: "Write `wordCount(s string) map[string]int` that counts word occurrences. Print the result for \"go is fun go is great\".", Template: "package main\n\nimport (\n\t\"fmt\"\n\t\"strings\"\n)\n\nfunc wordCount(s string) map[string]int {\n\t// Your code here\n}\n\nfunc main() {\n\tfmt.Println(wordCount(\"go is fun go is great\"))\n}", Hints: []string{"Use strings.Split", "Increment map[word]++"}},
	{ID: 11, Title: "Stack Implementation", Difficulty: "medium", Category: "data-structures", Points: 120, TimeBonus: 120, Description: "Implement a Stack using a struct with Push, Pop, and IsEmpty methods. Demonstrate by pushing 1,2,3 and popping all.", Template: "package main\n\nimport \"fmt\"\n\ntype Stack struct {\n\t// Your fields here\n}\n\nfunc (s *Stack) Push(val int) {\n\t// Your code\n}\n\nfunc (s *Stack) Pop() (int, bool) {\n\t// Your code\n}\n\nfunc (s *Stack) IsEmpty() bool {\n\t// Your code\n}\n\nfunc main() {\n\ts := &Stack{}\n\ts.Push(1)\n\ts.Push(2)\n\ts.Push(3)\n\tfor !s.IsEmpty() {\n\t\tval, _ := s.Pop()\n\t\tfmt.Println(val)\n\t}\n}", Hints: []string{"Use a slice as backing store", "Pop returns value and ok bool"}},
	{ID: 12, Title: "Goroutine Hello", Difficulty: "medium", Category: "concurrency", Points: 130, TimeBonus: 90, Description: "Launch 5 goroutines each printing their index. Use a WaitGroup to wait for all to finish.", Template: "package main\n\nimport (\n\t\"fmt\"\n\t\"sync\"\n)\n\nfunc main() {\n\tvar wg sync.WaitGroup\n\t// Launch 5 goroutines\n\twg.Wait()\n}", Hints: []string{"wg.Add(1) before go func", "defer wg.Done() inside goroutine"}},
	{ID: 13, Title: "Channel Pipeline", Difficulty: "medium", Category: "concurrency", Points: 150, TimeBonus: 120, Description: "Create a pipeline: generate numbers 1-5 on a channel, square them on another channel, print results.", Template: "package main\n\nimport \"fmt\"\n\nfunc generate(nums ...int) <-chan int {\n\t// Your code\n}\n\nfunc square(in <-chan int) <-chan int {\n\t// Your code\n}\n\nfunc main() {\n\tfor n := range square(generate(1, 2, 3, 4, 5)) {\n\t\tfmt.Println(n)\n\t}\n}", Hints: []string{"Create buffered or unbuffered channels", "Close channel when done"}},
	{ID: 14, Title: "Error Handling", Difficulty: "medium", Category: "errors", Points: 110, TimeBonus: 90, Description: "Write `divide(a, b float64) (float64, error)` that returns an error if b is 0. Test with divide(10,2) and divide(5,0).", Template: "package main\n\nimport (\n\t\"errors\"\n\t\"fmt\"\n)\n\nfunc divide(a, b float64) (float64, error) {\n\t// Your code\n}\n\nfunc main() {\n\t// Test both cases\n}", Hints: []string{"Use errors.New() for the error", "Return (0, err) on failure"}},
	{ID: 15, Title: "Interface Shape", Difficulty: "medium", Category: "interfaces", Points: 140, TimeBonus: 120, Description: "Define a Shape interface with Area() float64. Implement Circle and Rectangle. Print areas of a circle (r=5) and rectangle (3x4).", Template: "package main\n\nimport (\n\t\"fmt\"\n\t\"math\"\n)\n\ntype Shape interface {\n\tArea() float64\n}\n\ntype Circle struct{ Radius float64 }\ntype Rectangle struct{ Width, Height float64 }\n\n// Implement Area() for both\n\nfunc main() {\n\tshapes := []Shape{Circle{5}, Rectangle{3, 4}}\n\tfor _, s := range shapes {\n\t\tfmt.Printf(\"Area: %.2f\\n\", s.Area())\n\t}\n}", Hints: []string{"Circle area = \u03c0*r\u00b2", "Rectangle area = w*h"}},
	{ID: 16, Title: "Closure Counter", Difficulty: "medium", Category: "closures", Points: 110, TimeBonus: 90, Description: "Write `makeCounter()` that returns a function. Each call to the returned function increments and returns a counter.", Template: "package main\n\nimport \"fmt\"\n\nfunc makeCounter() func() int {\n\t// Your code\n}\n\nfunc main() {\n\tcounter := makeCounter()\n\tfmt.Println(counter()) // 1\n\tfmt.Println(counter()) // 2\n\tfmt.Println(counter()) // 3\n}", Hints: []string{"Use closure over a local variable"}},
	{ID: 17, Title: "JSON Marshal", Difficulty: "medium", Category: "json", Points: 120, TimeBonus: 90, Description: "Create a Person struct with Name and Age. Marshal it to JSON and print. Then unmarshal back and print.", Template: "package main\n\nimport (\n\t\"encoding/json\"\n\t\"fmt\"\n)\n\ntype Person struct {\n\tName string ", Hints: []string{"json.Marshal returns ([]byte, error)", "json.Unmarshal takes &target"}},
	{ID: 18, Title: "Variadic Sum", Difficulty: "medium", Category: "functions", Points: 100, TimeBonus: 60, Description: "Write `sum(nums ...int) int` that sums any number of integers. Print sum(1,2,3,4,5).", Template: "package main\n\nimport \"fmt\"\n\nfunc sum(nums ...int) int {\n\t// Your code\n}\n\nfunc main() {\n\tfmt.Println(sum(1, 2, 3, 4, 5))\n}", Hints: []string{"nums is a slice inside the function"}},
	{ID: 19, Title: "Mutex Counter", Difficulty: "medium", Category: "concurrency", Points: 150, TimeBonus: 120, Description: "Use a mutex-protected counter incremented by 100 goroutines (each by 1). Print the final count (should be 100).", Template: "package main\n\nimport (\n\t\"fmt\"\n\t\"sync\"\n)\n\ntype SafeCounter struct {\n\tmu    sync.Mutex\n\tcount int\n}\n\nfunc (c *SafeCounter) Inc() {\n\t// Your code\n}\n\nfunc main() {\n\tvar wg sync.WaitGroup\n\tc := &SafeCounter{}\n\tfor i := 0; i < 100; i++ {\n\t\twg.Add(1)\n\t\tgo func() {\n\t\t\tdefer wg.Done()\n\t\t\tc.Inc()\n\t\t}()\n\t}\n\twg.Wait()\n\tfmt.Println(c.count)\n}", Hints: []string{"Lock before modifying, unlock after"}},
	{ID: 20, Title: "Binary Search", Difficulty: "medium", Category: "algorithms", Points: 130, TimeBonus: 100, Description: "Implement `binarySearch(arr []int, target int) int` returning the index or -1. Test with [1,3,5,7,9,11], target=7.", Template: "package main\n\nimport \"fmt\"\n\nfunc binarySearch(arr []int, target int) int {\n\t// Your code\n}\n\nfunc main() {\n\tarr := []int{1, 3, 5, 7, 9, 11}\n\tfmt.Println(binarySearch(arr, 7))  // 3\n\tfmt.Println(binarySearch(arr, 4))  // -1\n}", Hints: []string{"Use low, high, mid pointers", "Slice must be sorted"}},
	{ID: 21, Title: "Linked List", Difficulty: "medium", Category: "data-structures", Points: 140, TimeBonus: 120, Description: "Implement a singly linked list with Append and Print methods. Append 1,2,3 and print.", Template: "package main\n\nimport \"fmt\"\n\ntype Node struct {\n\tVal  int\n\tNext *Node\n}\n\ntype LinkedList struct {\n\tHead *Node\n}\n\nfunc (l *LinkedList) Append(val int) {\n\t// Your code\n}\n\nfunc (l *LinkedList) Print() {\n\t// Your code\n}\n\nfunc main() {\n\tl := &LinkedList{}\n\tl.Append(1)\n\tl.Append(2)\n\tl.Append(3)\n\tl.Print()\n}", Hints: []string{"Traverse to find tail", "Print until Next is nil"}},
	{ID: 22, Title: "Select Statement", Difficulty: "medium", Category: "concurrency", Points: 145, TimeBonus: 100, Description: "Use select to receive from two channels. Send 'ping' on ch1 and 'pong' on ch2, use select to print whichever arrives.", Template: "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tch1 := make(chan string, 1)\n\tch2 := make(chan string, 1)\n\tch1 <- \"ping\"\n\tch2 <- \"pong\"\n\t// Use select to print from one of them\n}", Hints: []string{}},
	{ID: 23, Title: "Custom Error Type", Difficulty: "medium", Category: "errors", Points: 120, TimeBonus: 90, Description: "Create a ValidationError struct implementing the error interface. Use it in a validateAge function (age must be 0-150).", Template: "package main\n\nimport \"fmt\"\n\ntype ValidationError struct {\n\tField   string\n\tMessage string\n}\n\nfunc (e *ValidationError) Error() string {\n\t// Your code\n}\n\nfunc validateAge(age int) error {\n\t// Return ValidationError if invalid\n}\n\nfunc main() {\n\tfmt.Println(validateAge(25))\n\tfmt.Println(validateAge(200))\n}", Hints: []string{"Implement Error() string method", "Return nil for valid input"}},
	{ID: 24, Title: "Map Filter", Difficulty: "medium", Category: "functional", Points: 115, TimeBonus: 80, Description: "Write generic-style `filter(nums []int, fn func(int)bool) []int` and `mapSlice(nums []int, fn func(int)int) []int`. Filter evens then double them from [1..10].", Template: "package main\n\nimport \"fmt\"\n\nfunc filter(nums []int, fn func(int) bool) []int {\n\t// Your code\n}\n\nfunc mapSlice(nums []int, fn func(int) int) []int {\n\t// Your code\n}\n\nfunc main() {\n\tnums := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}\n\tevens := filter(nums, func(n int) bool { return n%2 == 0 })\n\tdoubled := mapSlice(evens, func(n int) int { return n * 2 })\n\tfmt.Println(doubled)\n}", Hints: []string{"Append to result slice when condition is true"}},
	{ID: 25, Title: "Stringer Interface", Difficulty: "medium", Category: "interfaces", Points: 110, TimeBonus: 80, Description: "Create a Temperature type (float64). Implement the fmt.Stringer interface to format as `23.5\u00b0C`.", Template: "package main\n\nimport \"fmt\"\n\ntype Temperature float64\n\nfunc (t Temperature) String() string {\n\t// Your code\n}\n\nfunc main() {\n\tt := Temperature(23.5)\n\tfmt.Println(t)\n}", Hints: []string{"fmt.Sprintf with %.1f format", "Return string with \u00b0C suffix"}},
	{ID: 26, Title: "Panic & Recover", Difficulty: "medium", Category: "errors", Points: 130, TimeBonus: 90, Description: "Write `safeDiv(a, b int) (result int, err error)` using defer/recover to catch division by zero panics.", Template: "package main\n\nimport \"fmt\"\n\nfunc safeDiv(a, b int) (result int, err error) {\n\tdefer func() {\n\t\t// recover here\n\t}()\n\treturn a / b, nil\n}\n\nfunc main() {\n\tfmt.Println(safeDiv(10, 2))\n\tfmt.Println(safeDiv(5, 0))\n}", Hints: []string{"recover() returns the panic value", "Set err = fmt.Errorf(...) in recover"}},
	{ID: 27, Title: "Goroutine Fan-Out", Difficulty: "medium", Category: "concurrency", Points: 160, TimeBonus: 120, Description: "Fan out: send jobs 1-5 to workers via channel. 3 workers read from the channel and print 'worker X got job Y'.", Template: "package main\n\nimport (\n\t\"fmt\"\n\t\"sync\"\n)\n\nfunc worker(id int, jobs <-chan int, wg *sync.WaitGroup) {\n\tdefer wg.Done()\n\tfor j := range jobs {\n\t\tfmt.Printf(\"worker %d got job %d\\n\", id, j)\n\t}\n}\n\nfunc main() {\n\tjobs := make(chan int, 5)\n\tvar wg sync.WaitGroup\n\t// Launch 3 workers, send 5 jobs\n\twg.Wait()\n}", Hints: []string{"Close(jobs) after sending so workers finish", "range over channel reads until closed"}},
	{ID: 28, Title: "Bubble Sort", Difficulty: "medium", Category: "algorithms", Points: 120, TimeBonus: 90, Description: "Implement bubble sort on a []int. Sort [5,3,8,1,2] and print.", Template: "package main\n\nimport \"fmt\"\n\nfunc bubbleSort(arr []int) {\n\t// Your code\n}\n\nfunc main() {\n\tarr := []int{5, 3, 8, 1, 2}\n\tbubbleSort(arr)\n\tfmt.Println(arr)\n}", Hints: []string{"Nested loops, swap if arr[j] > arr[j+1]"}},
	{ID: 29, Title: "String Builder", Difficulty: "medium", Category: "strings", Points: 100, TimeBonus: 60, Description: "Use strings.Builder to concatenate 'Go' 5 times efficiently. Print the result.", Template: "package main\n\nimport (\n\t\"fmt\"\n\t\"strings\"\n)\n\nfunc main() {\n\tvar sb strings.Builder\n\t// Your code\n\tfmt.Println(sb.String())\n}", Hints: []string{"sb.WriteString() adds to buffer"}},
	{ID: 30, Title: "Embed Interface", Difficulty: "medium", Category: "interfaces", Points: 135, TimeBonus: 100, Description: "Define Reader and Writer interfaces, embed them in ReadWriter. Implement all methods on a Buffer struct.", Template: "package main\n\nimport \"fmt\"\n\ntype Reader interface {\n\tRead() string\n}\n\ntype Writer interface {\n\tWrite(s string)\n}\n\ntype ReadWriter interface {\n\tReader\n\tWriter\n}\n\ntype Buffer struct {\n\tdata string\n}\n\n// Implement Read() and Write() on *Buffer\n\nfunc main() {\n\tvar rw ReadWriter = &Buffer{}\n\trw.Write(\"hello gopher\")\n\tfmt.Println(rw.Read())\n}", Hints: []string{"Buffer.data holds the string"}},
	{ID: 31, Title: "Generic Min", Difficulty: "hard", Category: "generics", Points: 200, TimeBonus: 150, Description: "Write a generic `Min[T constraints.Ordered](a, b T) T` function. Print Min(3,5) and Min(\"apple\",\"banana\").", Template: "package main\n\nimport (\n\t\"fmt\"\n\t\"golang.org/x/exp/constraints\"\n)\n\nfunc Min[T constraints.Ordered](a, b T) T {\n\t// Your code\n}\n\nfunc main() {\n\tfmt.Println(Min(3, 5))\n\tfmt.Println(Min(\"apple\", \"banana\"))\n}", Hints: []string{"Use type parameter [T constraints.Ordered]", "Simple if a < b comparison"}},
	{ID: 32, Title: "Ticker Timer", Difficulty: "hard", Category: "concurrency", Points: 180, TimeBonus: 120, Description: "Use time.Ticker to print 'tick' every 100ms, stop after 3 ticks using a done channel.", Template: "package main\n\nimport (\n\t\"fmt\"\n\t\"time\"\n)\n\nfunc main() {\n\tticker := time.NewTicker(100 * time.Millisecond)\n\tdone := make(chan bool)\n\tcount := 0\n\t// Your code using select\n}", Hints: []string{"select on ticker.C and done channel", "Send to done after 3 ticks"}},
	{ID: 33, Title: "Context Timeout", Difficulty: "hard", Category: "concurrency", Points: 190, TimeBonus: 150, Description: "Use context.WithTimeout (200ms) to cancel a slow operation. Print 'done' if completes, 'timeout' if not.", Template: "package main\n\nimport (\n\t\"context\"\n\t\"fmt\"\n\t\"time\"\n)\n\nfunc slowOp(ctx context.Context) error {\n\tselect {\n\tcase <-time.After(500 * time.Millisecond):\n\t\treturn nil\n\tcase <-ctx.Done():\n\t\treturn ctx.Err()\n\t}\n}\n\nfunc main() {\n\tctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)\n\tdefer cancel()\n\t// Your code\n}", Hints: []string{"Call slowOp(ctx) and check the error"}},
	{ID: 34, Title: "LRU Cache", Difficulty: "hard", Category: "data-structures", Points: 250, TimeBonus: 180, Description: "Implement an LRU cache with Get(key) and Put(key,val) with capacity 2. Show eviction behavior.", Template: "package main\n\nimport \"fmt\"\n\ntype LRUCache struct {\n\tcapacity int\n\t// Your fields\n}\n\nfunc NewLRU(cap int) *LRUCache {\n\treturn &LRUCache{capacity: cap}\n}\n\nfunc (c *LRUCache) Get(key int) int {\n\t// Return value or -1\n}\n\nfunc (c *LRUCache) Put(key, val int) {\n\t// Insert and evict LRU if over capacity\n}\n\nfunc main() {\n\tc := NewLRU(2)\n\tc.Put(1, 1)\n\tc.Put(2, 2)\n\tfmt.Println(c.Get(1)) // 1\n\tc.Put(3, 3)           // evicts key 2\n\tfmt.Println(c.Get(2)) // -1\n}", Hints: []string{"Use map + doubly linked list or ordered map"}},
	{ID: 35, Title: "Rate Limiter", Difficulty: "hard", Category: "concurrency", Points: 220, TimeBonus: 160, Description: "Build a token bucket rate limiter allowing 3 requests per second. Simulate 5 requests and show which succeed.", Template: "package main\n\nimport (\n\t\"fmt\"\n\t\"time\"\n)\n\ntype RateLimiter struct {\n\ttokens   int\n\tmax      int\n\trefillAt time.Time\n}\n\nfunc NewRateLimiter(max int) *RateLimiter {\n\treturn &RateLimiter{tokens: max, max: max, refillAt: time.Now().Add(time.Second)}\n}\n\nfunc (r *RateLimiter) Allow() bool {\n\t// Your code\n}\n\nfunc main() {\n\trl := NewRateLimiter(3)\n\tfor i := 1; i <= 5; i++ {\n\t\tif rl.Allow() {\n\t\t\tfmt.Printf(\"Request %d: allowed\\n\", i)\n\t\t} else {\n\t\t\tfmt.Printf(\"Request %d: rejected\\n\", i)\n\t\t}\n\t}\n}", Hints: []string{"Decrement tokens, refill after 1 second"}},
	{ID: 36, Title: "Trie Data Structure", Difficulty: "hard", Category: "data-structures", Points: 240, TimeBonus: 180, Description: "Implement a Trie with Insert and Search methods. Insert 'go', 'golang', 'gopher'. Search for 'go', 'goph', 'rust'.", Template: "package main\n\nimport \"fmt\"\n\ntype TrieNode struct {\n\tchildren map[rune]*TrieNode\n\tisEnd    bool\n}\n\ntype Trie struct {\n\troot *TrieNode\n}\n\nfunc NewTrie() *Trie {\n\treturn &Trie{root: &TrieNode{children: make(map[rune]*TrieNode)}}\n}\n\nfunc (t *Trie) Insert(word string) {\n\t// Your code\n}\n\nfunc (t *Trie) Search(word string) bool {\n\t// Your code\n}\n\nfunc main() {\n\tt := NewTrie()\n\tt.Insert(\"go\")\n\tt.Insert(\"golang\")\n\tt.Insert(\"gopher\")\n\tfmt.Println(t.Search(\"go\"))     // true\n\tfmt.Println(t.Search(\"goph\"))   // false\n\tfmt.Println(t.Search(\"rust\"))   // false\n}", Hints: []string{"Navigate children map by each rune", "isEnd marks a complete word"}},
	{ID: 37, Title: "Pipeline with Errors", Difficulty: "hard", Category: "concurrency", Points: 210, TimeBonus: 150, Description: "Build a 3-stage pipeline: generate ints, filter odds, square them. Handle errors gracefully using error channels.", Template: "package main\n\nimport \"fmt\"\n\nfunc generate(nums []int) <-chan int {\n\tch := make(chan int)\n\tgo func() {\n\t\tfor _, n := range nums {\n\t\t\tch <- n\n\t\t}\n\t\tclose(ch)\n\t}()\n\treturn ch\n}\n\nfunc filterOdd(in <-chan int) <-chan int {\n\t// Your code\n}\n\nfunc squareIt(in <-chan int) <-chan int {\n\t// Your code\n}\n\nfunc main() {\n\tfor n := range squareIt(filterOdd(generate([]int{1,2,3,4,5,6,7,8,9,10}))) {\n\t\tfmt.Println(n)\n\t}\n}", Hints: []string{"Each stage reads from in, writes to out", "Close out channel when in is exhausted"}},
	{ID: 38, Title: "Observer Pattern", Difficulty: "hard", Category: "patterns", Points: 200, TimeBonus: 140, Description: "Implement the Observer pattern: EventBus with Subscribe and Publish. Subscribe 2 listeners to 'login' event, publish and show both fire.", Template: "package main\n\nimport \"fmt\"\n\ntype Handler func(data interface{})\n\ntype EventBus struct {\n\t// Your fields\n}\n\nfunc (e *EventBus) Subscribe(event string, handler Handler) {\n\t// Your code\n}\n\nfunc (e *EventBus) Publish(event string, data interface{}) {\n\t// Your code\n}\n\nfunc main() {\n\tbus := &EventBus{}\n\tbus.Subscribe(\"login\", func(d interface{}) { fmt.Println(\"Logger:\", d) })\n\tbus.Subscribe(\"login\", func(d interface{}) { fmt.Println(\"Notifier:\", d) })\n\tbus.Publish(\"login\", \"user123\")\n}", Hints: []string{"Use map[string][]Handler", "Range over handlers on Publish"}},
	{ID: 39, Title: "Worker Pool", Difficulty: "hard", Category: "concurrency", Points: 220, TimeBonus: 160, Description: "Implement a worker pool of 3 goroutines processing 10 jobs. Collect all results and print.", Template: "package main\n\nimport (\n\t\"fmt\"\n\t\"sync\"\n)\n\nfunc worker(id int, jobs <-chan int, results chan<- int, wg *sync.WaitGroup) {\n\tdefer wg.Done()\n\tfor j := range jobs {\n\t\tresults <- j * j // square the job\n\t}\n}\n\nfunc main() {\n\tjobs := make(chan int, 10)\n\tresults := make(chan int, 10)\n\tvar wg sync.WaitGroup\n\n\t// Start 3 workers, send 10 jobs, collect results\n\twg.Wait()\n\tclose(results)\n\tfor r := range results {\n\t\tfmt.Println(r)\n\t}\n}", Hints: []string{"wg.Add(3) for 3 workers", "Close jobs channel after sending all"}},
	{ID: 40, Title: "Memoize Function", Difficulty: "hard", Category: "functional", Points: 200, TimeBonus: 140, Description: "Write a `memoize` function that caches results of an expensive int->int function. Demonstrate with fibonacci.", Template: "package main\n\nimport \"fmt\"\n\nfunc memoize(fn func(int) int) func(int) int {\n\tcache := map[int]int{}\n\treturn func(n int) int {\n\t\t// Your code\n\t}\n}\n\nfunc main() {\n\tvar fib func(int) int\n\tfib = memoize(func(n int) int {\n\t\tif n <= 1 {\n\t\t\treturn n\n\t\t}\n\t\treturn fib(n-1) + fib(n-2)\n\t})\n\tfmt.Println(fib(40))\n}", Hints: []string{"Check cache before calling fn", "Store result in cache after computing"}},
	{ID: 41, Title: "String Anagram", Difficulty: "easy", Category: "strings", Points: 75, TimeBonus: 60, Description: "Write `isAnagram(s, t string) bool`. Print isAnagram(\"anagram\", \"nagaram\").", Template: "package main\n\nimport \"fmt\"\n\nfunc isAnagram(s, t string) bool {\n\t// Your code\n}\n\nfunc main() {\n\tfmt.Println(isAnagram(\"anagram\", \"nagaram\"))\n}", Hints: []string{"Sort both strings", "Or use a frequency map"}},
	{ID: 42, Title: "Remove Duplicates", Difficulty: "easy", Category: "slices", Points: 70, TimeBonus: 45, Description: "Write `removeDups(nums []int) []int`. Print removeDups([]int{1,1,2,3,3,4}).", Template: "package main\n\nimport \"fmt\"\n\nfunc removeDups(nums []int) []int {\n\t// Your code\n}\n\nfunc main() {\n\tfmt.Println(removeDups([]int{1,1,2,3,3,4}))\n}", Hints: []string{"Use a map to track seen values"}},
	{ID: 43, Title: "Two Sum", Difficulty: "easy", Category: "algorithms", Points: 85, TimeBonus: 60, Description: "Write `twoSum(nums []int, target int) (int,int)` returning indices. Print twoSum([]int{2,7,11,15}, 9).", Template: "package main\n\nimport \"fmt\"\n\nfunc twoSum(nums []int, target int) (int,int) {\n\t// Your code\n}\n\nfunc main() {\n\ta,b := twoSum([]int{2,7,11,15}, 9)\n\tfmt.Println(a, b)\n}", Hints: []string{"Use a map: complement -> index"}},
	{ID: 44, Title: "Roman to Int", Difficulty: "medium", Category: "strings", Points: 130, TimeBonus: 90, Description: "Write `romanToInt(s string) int`. Convert 'XIV' and 'MCMXCIX'.", Template: "package main\n\nimport \"fmt\"\n\nfunc romanToInt(s string) int {\n\t// Your code\n}\n\nfunc main() {\n\tfmt.Println(romanToInt(\"XIV\"))\n\tfmt.Println(romanToInt(\"MCMXCIX\"))\n}", Hints: []string{"Map each symbol to value", "If smaller before larger, subtract"}},
	{ID: 45, Title: "Valid Brackets", Difficulty: "medium", Category: "data-structures", Points: 120, TimeBonus: 90, Description: "Write `isValid(s string) bool` for bracket matching '([{}])'. Test '([])' and '([)]'.", Template: "package main\n\nimport \"fmt\"\n\nfunc isValid(s string) bool {\n\t// Your code\n}\n\nfunc main() {\n\tfmt.Println(isValid(\"([])\"))\n\tfmt.Println(isValid(\"([)]\"))\n}", Hints: []string{"Use a stack", "Push open brackets, pop and match on close"}},
	{ID: 46, Title: "Merge Sorted Arrays", Difficulty: "medium", Category: "algorithms", Points: 115, TimeBonus: 80, Description: "Write `merge(a, b []int) []int`. Merge [1,3,5] and [2,4,6].", Template: "package main\n\nimport \"fmt\"\n\nfunc merge(a, b []int) []int {\n\t// Your code\n}\n\nfunc main() {\n\tfmt.Println(merge([]int{1,3,5}, []int{2,4,6}))\n}", Hints: []string{"Two pointer approach", "Append remaining elements at end"}},
	{ID: 47, Title: "Power Function", Difficulty: "easy", Category: "algorithms", Points: 65, TimeBonus: 45, Description: "Write `pow(base, exp int) int` without using math.Pow. Print pow(2,10).", Template: "package main\n\nimport \"fmt\"\n\nfunc pow(base, exp int) int {\n\t// Your code\n}\n\nfunc main() {\n\tfmt.Println(pow(2, 10))\n}", Hints: []string{"Multiply base by itself exp times", "Or use fast exponentiation"}},
	{ID: 48, Title: "GCD & LCM", Difficulty: "easy", Category: "algorithms", Points: 75, TimeBonus: 50, Description: "Write `gcd(a,b int) int` and `lcm(a,b int) int`. Print gcd(48,18) and lcm(4,6).", Template: "package main\n\nimport \"fmt\"\n\nfunc gcd(a, b int) int {\n\t// Your code\n}\n\nfunc lcm(a, b int) int {\n\t// Your code\n}\n\nfunc main() {\n\tfmt.Println(gcd(48, 18))\n\tfmt.Println(lcm(4, 6))\n}", Hints: []string{"Use Euclidean algorithm for GCD", "LCM = a*b/gcd(a,b)"}},
	{ID: 49, Title: "Prime Sieve", Difficulty: "medium", Category: "algorithms", Points: 140, TimeBonus: 100, Description: "Implement Sieve of Eratosthenes to find all primes up to 50.", Template: "package main\n\nimport \"fmt\"\n\nfunc sieve(n int) []int {\n\t// Your code\n}\n\nfunc main() {\n\tfmt.Println(sieve(50))\n}", Hints: []string{"Boolean array, mark composites as false", "Start from 2, mark multiples"}},
	{ID: 50, Title: "Matrix Transpose", Difficulty: "medium", Category: "algorithms", Points: 125, TimeBonus: 90, Description: "Write `transpose(matrix [][]int) [][]int`. Transpose [[1,2,3],[4,5,6]].", Template: "package main\n\nimport \"fmt\"\n\nfunc transpose(matrix [][]int) [][]int {\n\t// Your code\n}\n\nfunc main() {\n\tm := [][]int{{1,2,3},{4,5,6}}\n\tfor _, row := range transpose(m) {\n\t\tfmt.Println(row)\n\t}\n}", Hints: []string{"Result[j][i] = input[i][j]"}},
	{ID: 51, Title: "Flatten Slice", Difficulty: "easy", Category: "slices", Points: 70, TimeBonus: 45, Description: "Write `flatten(nested [][]int) []int`. Flatten [[1,2],[3,4],[5]].", Template: "package main\n\nimport \"fmt\"\n\nfunc flatten(nested [][]int) []int {\n\t// Your code\n}\n\nfunc main() {\n\tfmt.Println(flatten([][]int{{1,2},{3,4},{5}}))\n}", Hints: []string{"Append each sub-slice to result"}},
	{ID: 52, Title: "String Compression", Difficulty: "medium", Category: "strings", Points: 120, TimeBonus: 80, Description: "Write `compress(s string) string` - 'aabcc' becomes 'a2bc2'. Print compress('aabbbcccc').", Template: "package main\n\nimport \"fmt\"\n\nfunc compress(s string) string {\n\t// Your code\n}\n\nfunc main() {\n\tfmt.Println(compress(\"aabbbcccc\"))\n}", Hints: []string{"Count consecutive chars", "Only add count if > 1"}},
	{ID: 53, Title: "Rotate Slice", Difficulty: "easy", Category: "slices", Points: 75, TimeBonus: 50, Description: "Write `rotate(nums []int, k int) []int` - rotate right by k. Rotate [1,2,3,4,5] by 2.", Template: "package main\n\nimport \"fmt\"\n\nfunc rotate(nums []int, k int) []int {\n\t// Your code\n}\n\nfunc main() {\n\tfmt.Println(rotate([]int{1,2,3,4,5}, 2))\n}", Hints: []string{"k = k % len(nums)", "Slice rearrangement: nums[n-k:] + nums[:n-k]"}},
	{ID: 54, Title: "Type Assertion", Difficulty: "medium", Category: "interfaces", Points: 110, TimeBonus: 75, Description: "Write `describe(i interface{}) string` that returns 'int:N', 'string:S', or 'unknown'. Test with 42, 'hello', 3.14.", Template: "package main\n\nimport \"fmt\"\n\nfunc describe(i interface{}) string {\n\t// Use type switch\n}\n\nfunc main() {\n\tfmt.Println(describe(42))\n\tfmt.Println(describe(\"hello\"))\n\tfmt.Println(describe(3.14))\n}", Hints: []string{}},
	{ID: 55, Title: "defer Stack Order", Difficulty: "easy", Category: "basics", Points: 65, TimeBonus: 40, Description: "Using 3 defers, demonstrate LIFO order. Defers should print 3, 2, 1.", Template: "package main\n\nimport \"fmt\"\n\nfunc main() {\n\t// Three defers printing 1, 2, 3 in reverse\n}", Hints: []string{"Defers execute LIFO (last in, first out)"}},
	{ID: 56, Title: "Functional Options", Difficulty: "hard", Category: "patterns", Points: 200, TimeBonus: 150, Description: "Implement functional options pattern for a Server struct (host, port, timeout). Create server with WithHost, WithPort options.", Template: "package main\n\nimport \"fmt\"\n\ntype Server struct {\n\thost    string\n\tport    int\n\ttimeout int\n}\n\ntype Option func(*Server)\n\nfunc WithHost(h string) Option {\n\t// Your code\n}\n\nfunc WithPort(p int) Option {\n\t// Your code\n}\n\nfunc NewServer(opts ...Option) *Server {\n\ts := &Server{host:\"localhost\", port:8080, timeout:30}\n\t// Apply options\n\treturn s\n}\n\nfunc main() {\n\ts := NewServer(WithHost(\"example.com\"), WithPort(9090))\n\tfmt.Printf(\"%s:%d\\n\", s.host, s.port)\n}", Hints: []string{"Option is a func(*Server)"}},
	{ID: 57, Title: "Semaphore", Difficulty: "hard", Category: "concurrency", Points: 190, TimeBonus: 140, Description: "Implement a semaphore using a buffered channel limiting concurrency to 3. Show 6 goroutines with max 3 concurrent.", Template: "package main\n\nimport (\n\t\"fmt\"\n\t\"sync\"\n\t\"time\"\n)\n\ntype Semaphore chan struct{}\n\nfunc (s Semaphore) Acquire() { s <- struct{}{} }\nfunc (s Semaphore) Release() { <-s }\n\nfunc main() {\n\tsem := make(Semaphore, 3)\n\tvar wg sync.WaitGroup\n\tfor i := 1; i <= 6; i++ {\n\t\twg.Add(1)\n\t\tgo func(id int) {\n\t\t\tdefer wg.Done()\n\t\t\tsem.Acquire()\n\t\t\tdefer sem.Release()\n\t\t\tfmt.Printf(\"goroutine %d running\\n\", id)\n\t\t\ttime.Sleep(50 * time.Millisecond)\n\t\t}(i)\n\t}\n\twg.Wait()\n\tfmt.Println(\"done\")\n}", Hints: []string{"Buffered channel of size N acts as semaphore"}},
	{ID: 58, Title: "Merge Sort", Difficulty: "hard", Category: "algorithms", Points: 210, TimeBonus: 150, Description: "Implement merge sort on []int. Sort [38,27,43,3,9,82,10] and print.", Template: "package main\n\nimport \"fmt\"\n\nfunc mergeSort(arr []int) []int {\n\t// Your code\n}\n\nfunc main() {\n\tarr := []int{38,27,43,3,9,82,10}\n\tfmt.Println(mergeSort(arr))\n}", Hints: []string{"Base case: len <= 1", "Split, recurse, merge sorted halves"}},
	{ID: 59, Title: "Quick Sort", Difficulty: "hard", Category: "algorithms", Points: 210, TimeBonus: 150, Description: "Implement quick sort on []int in-place. Sort [3,6,8,10,1,2,1] and print.", Template: "package main\n\nimport \"fmt\"\n\nfunc quickSort(arr []int, low, high int) {\n\t// Your code\n}\n\nfunc partition(arr []int, low, high int) int {\n\t// Your code\n}\n\nfunc main() {\n\tarr := []int{3,6,8,10,1,2,1}\n\tquickSort(arr, 0, len(arr)-1)\n\tfmt.Println(arr)\n}", Hints: []string{"Pick pivot (last element)", "Partition around pivot, recurse"}},
	{ID: 60, Title: "Graph BFS", Difficulty: "hard", Category: "algorithms", Points: 230, TimeBonus: 170, Description: "Implement BFS on an adjacency list graph. Find shortest path from node 0 to node 4 in a 5-node graph.", Template: "package main\n\nimport \"fmt\"\n\nfunc bfs(graph map[int][]int, start, end int) []int {\n\t// Return path\n}\n\nfunc main() {\n\tgraph := map[int][]int{\n\t\t0: {1, 2},\n\t\t1: {0, 3},\n\t\t2: {0, 4},\n\t\t3: {1, 4},\n\t\t4: {2, 3},\n\t}\n\tfmt.Println(bfs(graph, 0, 4))\n}", Hints: []string{"Use a queue (slice)", "Track visited and parent nodes"}},
	{ID: 61, Title: "Stdin Reader", Difficulty: "easy", Category: "io", Points: 70, TimeBonus: 50, Description: "Read a line from stdin using bufio.Scanner. For the game, just simulate: set input to a string and process it. Print the uppercased version.", Template: "package main\n\nimport (\n\t\"fmt\"\n\t\"strings\"\n)\n\nfunc main() {\n\tinput := \"hello gopher\"\n\tfmt.Println(strings.ToUpper(input))\n}", Hints: []string{"strings.ToUpper converts to uppercase"}},
	{ID: 62, Title: "Struct Embedding", Difficulty: "medium", Category: "oop", Points: 125, TimeBonus: 90, Description: "Embed Animal struct in Dog. Animal has Speak() method. Override it in Dog. Demonstrate polymorphism.", Template: "package main\n\nimport \"fmt\"\n\ntype Animal struct{ Name string }\nfunc (a Animal) Speak() string { return a.Name + \" speaks\" }\n\ntype Dog struct {\n\tAnimal\n\t// Your fields\n}\n\nfunc (d Dog) Speak() string {\n\t// Override\n}\n\nfunc main() {\n\td := Dog{Animal: Animal{Name: \"Rex\"}}\n\tfmt.Println(d.Speak())\n\tfmt.Println(d.Animal.Speak())\n}", Hints: []string{"Embedded struct fields are promoted", "d.Animal.Speak() calls parent"}},
	{ID: 63, Title: "Map Inversion", Difficulty: "easy", Category: "maps", Points: 75, TimeBonus: 50, Description: "Write `invertMap(m map[string]int) map[int]string`. Invert {'a':1,'b':2,'c':3}.", Template: "package main\n\nimport \"fmt\"\n\nfunc invertMap(m map[string]int) map[int]string {\n\t// Your code\n}\n\nfunc main() {\n\tresult := invertMap(map[string]int{\"a\":1,\"b\":2,\"c\":3})\n\tfmt.Println(result)\n}", Hints: []string{"Iterate map, swap key and value"}},
	{ID: 64, Title: "Pipe Channels", Difficulty: "hard", Category: "concurrency", Points: 195, TimeBonus: 140, Description: "Write a function `merge(cs ...<-chan int) <-chan int` that merges multiple channels into one.", Template: "package main\n\nimport (\n\t\"fmt\"\n\t\"sync\"\n)\n\nfunc merge(cs ...<-chan int) <-chan int {\n\tvar wg sync.WaitGroup\n\tout := make(chan int)\n\t// Your code - start goroutine for each channel\n\tgo func() { wg.Wait(); close(out) }()\n\treturn out\n}\n\nfunc fromSlice(nums []int) <-chan int {\n\tch := make(chan int)\n\tgo func() {\n\t\tfor _, n := range nums { ch <- n }\n\t\tclose(ch)\n\t}()\n\treturn ch\n}\n\nfunc main() {\n\tch1 := fromSlice([]int{1,2,3})\n\tch2 := fromSlice([]int{4,5,6})\n\tfor n := range merge(ch1, ch2) {\n\t\tfmt.Println(n)\n\t}\n}", Hints: []string{"wg.Add per channel, goroutine forwards and calls wg.Done"}},
	{ID: 65, Title: "Error Wrapping", Difficulty: "medium", Category: "errors", Points: 130, TimeBonus: 90, Description: "Use fmt.Errorf with %w to wrap errors. Create a chain: dbErr -> queryErr -> serviceErr. Unwrap and print each.", Template: "package main\n\nimport (\n\t\"errors\"\n\t\"fmt\"\n)\n\nfunc main() {\n\tdbErr := errors.New(\"connection refused\")\n\tqueryErr := fmt.Errorf(\"query failed: %w\", dbErr)\n\tserviceErr := fmt.Errorf(\"service error: %w\", queryErr)\n\tfmt.Println(serviceErr)\n\tfmt.Println(errors.Is(serviceErr, dbErr))\n\tvar target *errors.errorString\n\t_ = target\n\t// Unwrap the chain\n}", Hints: []string{"errors.Is traverses the chain", "errors.Unwrap goes one level"}},
	{ID: 66, Title: "Sort Custom Type", Difficulty: "medium", Category: "sorting", Points: 130, TimeBonus: 90, Description: "Sort []Person by Age using sort.Slice. Print sorted slice of {Alice,30}, {Bob,25}, {Charlie,35}.", Template: "package main\n\nimport (\n\t\"fmt\"\n\t\"sort\"\n)\n\ntype Person struct {\n\tName string\n\tAge  int\n}\n\nfunc main() {\n\tpeople := []Person{{\"Alice\",30},{\"Bob\",25},{\"Charlie\",35}}\n\t// Sort by age\n\tfor _, p := range people {\n\t\tfmt.Printf(\"%s: %d\\n\", p.Name, p.Age)\n\t}\n}", Hints: []string{}},
	{ID: 67, Title: "String Split Join", Difficulty: "easy", Category: "strings", Points: 65, TimeBonus: 40, Description: "Split 'the quick brown fox' by space, reverse the slice of words, join back with '-'. Print result.", Template: "package main\n\nimport (\n\t\"fmt\"\n\t\"strings\"\n)\n\nfunc main() {\n\ts := \"the quick brown fox\"\n\t// Split, reverse, join\n}", Hints: []string{"strings.Split and strings.Join", "Reverse using two pointers"}},
	{ID: 68, Title: "Atomic Counter", Difficulty: "hard", Category: "concurrency", Points: 185, TimeBonus: 130, Description: "Use sync/atomic to safely increment a counter from 1000 goroutines. Print final value (should be 1000).", Template: "package main\n\nimport (\n\t\"fmt\"\n\t\"sync\"\n\t\"sync/atomic\"\n)\n\nfunc main() {\n\tvar counter int64\n\tvar wg sync.WaitGroup\n\tfor i := 0; i < 1000; i++ {\n\t\twg.Add(1)\n\t\tgo func() {\n\t\t\tdefer wg.Done()\n\t\t\t// Atomically increment counter\n\t\t}()\n\t}\n\twg.Wait()\n\tfmt.Println(counter)\n}", Hints: []string{"atomic.AddInt64(&counter, 1)"}},
	{ID: 69, Title: "Regex Match", Difficulty: "medium", Category: "strings", Points: 120, TimeBonus: 80, Description: "Use regexp to find all email addresses in a string. Print matches from 'contact alice@go.dev or bob@example.com'.", Template: "package main\n\nimport (\n\t\"fmt\"\n\t\"regexp\"\n)\n\nfunc main() {\n\ttext := \"contact alice@go.dev or bob@example.com\"\n\tre := regexp.MustCompile(\"[a-zA-Z0-9._%+\\\\-]+@[a-zA-Z0-9.\\\\-]+\\\\.[a-zA-Z]{2,}\")\n\tfmt.Println(re.FindAllString(text, -1))\n}", Hints: []string{"regexp.MustCompile then FindAllString"}},
	{ID: 70, Title: "Time Formatting", Difficulty: "easy", Category: "stdlib", Points: 65, TimeBonus: 40, Description: "Format time.Now() as 'Mon Jan 2 2006 15:04:05'. Print the result.", Template: "package main\n\nimport (\n\t\"fmt\"\n\t\"time\"\n)\n\nfunc main() {\n\tnow := time.Now()\n\t// Format and print\n}", Hints: []string{"Go uses reference time: Mon Jan 2 15:04:05 2006", "now.Format(layout)"}},
	{ID: 71, Title: "Defer Cleanup", Difficulty: "easy", Category: "basics", Points: 70, TimeBonus: 45, Description: "Simulate file open/close with defer. Print 'opening file', work, then 'closing file' via defer.", Template: "package main\n\nimport \"fmt\"\n\nfunc processFile(name string) {\n\tfmt.Println(\"opening\", name)\n\tdefer fmt.Println(\"closing\", name)\n\tfmt.Println(\"working on\", name)\n}\n\nfunc main() {\n\tprocessFile(\"data.txt\")\n}", Hints: []string{"defer runs when function returns"}},
	{ID: 72, Title: "Buffered Channel", Difficulty: "medium", Category: "concurrency", Points: 130, TimeBonus: 90, Description: "Create a buffered channel of size 3. Send 3 values without a receiver. Print all 3.", Template: "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tch := make(chan int, 3)\n\t// Send 3 values and print them\n}", Hints: []string{"Buffered channels don't block until full"}},
	{ID: 73, Title: "Nil Interface Check", Difficulty: "medium", Category: "interfaces", Points: 115, TimeBonus: 80, Description: "Write `isNil(i interface{}) bool` that correctly detects nil interfaces. Show the typed nil vs untyped nil issue.", Template: "package main\n\nimport (\n\t\"fmt\"\n\t\"reflect\"\n)\n\nfunc isNil(i interface{}) bool {\n\tif i == nil {\n\t\treturn true\n\t}\n\tv := reflect.ValueOf(i)\n\treturn v.Kind() == reflect.Ptr && v.IsNil()\n}\n\nfunc main() {\n\tvar p *int = nil\n\tfmt.Println(isNil(nil))\n\tfmt.Println(isNil(p))\n}", Hints: []string{"Use reflect.ValueOf to detect typed nils"}},
	{ID: 74, Title: "Struct Tags", Difficulty: "medium", Category: "reflection", Points: 125, TimeBonus: 85, Description: "Use reflection to print all struct field names and their json tags for a User{Name,Email,Age} struct.", Template: "package main\n\nimport (\n\t\"fmt\"\n\t\"reflect\"\n)\n\ntype User struct {\n\tName  string `json:\"name\"`\n\tEmail string `json:\"email\"`\n\tAge   int    `json:\"age\"`\n}\n\nfunc main() {\n\tt := reflect.TypeOf(User{})\n\tfor i := 0; i < t.NumField(); i++ {\n\t\tf := t.Field(i)\n\t\tfmt.Printf(\"%s -> %s\\n\", f.Name, f.Tag.Get(\"json\"))\n\t}\n}", Hints: []string{"reflect.TypeOf, t.NumField(), t.Field(i).Tag.Get(\"json\")"}},
	{ID: 75, Title: "Once Init", Difficulty: "medium", Category: "concurrency", Points: 135, TimeBonus: 90, Description: "Use sync.Once to ensure an init function runs exactly once, even from 10 goroutines.", Template: "package main\n\nimport (\n\t\"fmt\"\n\t\"sync\"\n)\n\nvar (\n\tonce     sync.Once\n\tinstance string\n)\n\nfunc initialize() {\n\tinstance = \"initialized\"\n\tfmt.Println(\"init ran\")\n}\n\nfunc main() {\n\tvar wg sync.WaitGroup\n\tfor i := 0; i < 10; i++ {\n\t\twg.Add(1)\n\t\tgo func() {\n\t\t\tdefer wg.Done()\n\t\t\tonce.Do(initialize)\n\t\t}()\n\t}\n\twg.Wait()\n\tfmt.Println(instance)\n}", Hints: []string{"once.Do(fn) guarantees fn runs once"}},
	{ID: 76, Title: "HTTP Handler", Difficulty: "hard", Category: "web", Points: 200, TimeBonus: 150, Description: "Write a simple HTTP handler function that returns JSON `{\"status\":\"ok\"}`. Register on /health. (Just write the handler logic, don't start server.)", Template: "package main\n\nimport (\n\t\"encoding/json\"\n\t\"fmt\"\n\t\"net/http\"\n\t\"net/http/httptest\"\n)\n\nfunc healthHandler(w http.ResponseWriter, r *http.Request) {\n\t// Write JSON response\n}\n\nfunc main() {\n\treq := httptest.NewRequest(\"GET\", \"/health\", nil)\n\tw := httptest.NewRecorder()\n\thealthHandler(w, req)\n\tfmt.Println(w.Body.String())\n}", Hints: []string{"w.Header().Set(\"Content-Type\", \"application/json\")", "json.NewEncoder(w).Encode(map)"}},
	{ID: 77, Title: "Strings Fields", Difficulty: "easy", Category: "strings", Points: 60, TimeBonus: 35, Description: "Use strings.Fields to split '  hello   world  go  ' and print each word on a new line.", Template: "package main\n\nimport (\n\t\"fmt\"\n\t\"strings\"\n)\n\nfunc main() {\n\ts := \"  hello   world  go  \"\n\t// Your code\n}", Hints: []string{"strings.Fields handles multiple spaces"}},
	{ID: 78, Title: "Strconv Parse", Difficulty: "easy", Category: "stdlib", Points: 65, TimeBonus: 40, Description: "Parse '42' and '3.14' from strings to int and float64. Print their sum as float64.", Template: "package main\n\nimport (\n\t\"fmt\"\n\t\"strconv\"\n)\n\nfunc main() {\n\t// Parse and sum\n}", Hints: []string{"strconv.Atoi for int, strconv.ParseFloat for float"}},
	{ID: 79, Title: "Byte Slice Ops", Difficulty: "easy", Category: "strings", Points: 65, TimeBonus: 40, Description: "Convert string to []byte, change first byte to uppercase manually, convert back to string. Test with 'gopher'.", Template: "package main\n\nimport \"fmt\"\n\nfunc main() {\n\ts := \"gopher\"\n\t// Convert, modify, convert back\n}", Hints: []string{"[]byte(s) converts", "Subtract 32 for ASCII uppercase"}},
	{ID: 80, Title: "Named Return Values", Difficulty: "medium", Category: "functions", Points: 100, TimeBonus: 70, Description: "Write `minMax(nums []int) (min, max int)` using named return values. Print for [3,1,4,1,5,9].", Template: "package main\n\nimport \"fmt\"\n\nfunc minMax(nums []int) (min, max int) {\n\tmin, max = nums[0], nums[0]\n\t// Your code\n}\n\nfunc main() {\n\tmin, max := minMax([]int{3,1,4,1,5,9})\n\tfmt.Println(min, max)\n}", Hints: []string{"Named returns are initialized, bare return works"}},
	{ID: 81, Title: "Sliding Window", Difficulty: "hard", Category: "algorithms", Points: 220, TimeBonus: 160, Description: "Find the maximum sum of a subarray of size k. Test with [2,1,5,1,3,2], k=3 (answer: 9).", Template: "package main\n\nimport \"fmt\"\n\nfunc maxSubarraySum(arr []int, k int) int {\n\t// Sliding window\n}\n\nfunc main() {\n\tfmt.Println(maxSubarraySum([]int{2,1,5,1,3,2}, 3))\n}", Hints: []string{"Compute first window sum", "Slide: add arr[i], subtract arr[i-k]"}},
	{ID: 82, Title: "Depth-First Search", Difficulty: "hard", Category: "algorithms", Points: 230, TimeBonus: 170, Description: "Implement DFS on a graph. Print DFS traversal starting from node 0 for a 5-node graph.", Template: "package main\n\nimport \"fmt\"\n\nfunc dfs(graph map[int][]int, node int, visited map[int]bool) {\n\t// Your code\n}\n\nfunc main() {\n\tgraph := map[int][]int{0:{1,2},1:{0,3,4},2:{0},3:{1},4:{1}}\n\tvisited := map[int]bool{}\n\tdfs(graph, 0, visited)\n}", Hints: []string{"Mark visited, recurse on unvisited neighbors"}},
	{ID: 83, Title: "Struct Method Chaining", Difficulty: "medium", Category: "oop", Points: 135, TimeBonus: 95, Description: "Implement a Builder pattern for a Query struct with Table, Where, Limit methods that return *Query for chaining.", Template: "package main\n\nimport \"fmt\"\n\ntype Query struct {\n\ttable string\n\twhere string\n\tlimit int\n}\n\nfunc (q *Query) Table(t string) *Query { q.table = t; return q }\nfunc (q *Query) Where(w string) *Query { q.where = w; return q }\nfunc (q *Query) Limit(l int) *Query   { q.limit = l; return q }\n\nfunc (q *Query) Build() string {\n\t// Return SQL-like string\n}\n\nfunc main() {\n\tq := (&Query{}).Table(\"users\").Where(\"age > 18\").Limit(10)\n\tfmt.Println(q.Build())\n}", Hints: []string{"Build returns formatted SELECT ... FROM ... WHERE ... LIMIT ..."}},
	{ID: 84, Title: "Recursive Tree", Difficulty: "hard", Category: "data-structures", Points: 240, TimeBonus: 170, Description: "Implement a BST with Insert and InOrder traversal. Insert [5,3,7,1,4] and print in-order.", Template: "package main\n\nimport \"fmt\"\n\ntype TreeNode struct {\n\tVal         int\n\tLeft, Right *TreeNode\n}\n\nfunc insert(root *TreeNode, val int) *TreeNode {\n\t// Your code\n}\n\nfunc inOrder(root *TreeNode) {\n\t// Your code\n}\n\nfunc main() {\n\tvar root *TreeNode\n\tfor _, v := range []int{5,3,7,1,4} {\n\t\troot = insert(root, v)\n\t}\n\tinOrder(root)\n}", Hints: []string{"If root==nil, create node", "If val < root.Val go left, else right"}},
	{ID: 85, Title: "Concurrent Map", Difficulty: "hard", Category: "concurrency", Points: 200, TimeBonus: 150, Description: "Use sync.Map to safely read/write from multiple goroutines. Store and retrieve 5 key-value pairs concurrently.", Template: "package main\n\nimport (\n\t\"fmt\"\n\t\"sync\"\n)\n\nfunc main() {\n\tvar m sync.Map\n\tvar wg sync.WaitGroup\n\tfor i := 0; i < 5; i++ {\n\t\twg.Add(1)\n\t\tgo func(n int) {\n\t\t\tdefer wg.Done()\n\t\t\tm.Store(n, n*n)\n\t\t}(i)\n\t}\n\twg.Wait()\n\tm.Range(func(k, v interface{}) bool {\n\t\tfmt.Printf(\"%v: %v\\n\", k, v)\n\t\treturn true\n\t})\n}", Hints: []string{"sync.Map is safe for concurrent use without locks"}},
	{ID: 86, Title: "String Replace", Difficulty: "easy", Category: "strings", Points: 55, TimeBonus: 30, Description: "Replace all occurrences of 'Go' with 'Golang' in 'Go is great! Go is fast! Go rocks!'.", Template: "package main\n\nimport (\n\t\"fmt\"\n\t\"strings\"\n)\n\nfunc main() {\n\ts := \"Go is great! Go is fast! Go rocks!\"\n\t// Your code\n}", Hints: []string{"strings.ReplaceAll(s, old, new)"}},
	{ID: 87, Title: "Iota Enum", Difficulty: "easy", Category: "basics", Points: 65, TimeBonus: 40, Description: "Define a Weekday type using iota (Monday=1..Sunday=7). Print Monday and Friday with their values.", Template: "package main\n\nimport \"fmt\"\n\ntype Weekday int\n\nconst (\n\tMonday Weekday = iota + 1\n\t// Rest of days\n)\n\nfunc main() {\n\tfmt.Println(Monday, Friday)\n}", Hints: []string{"iota increments for each const in a group"}},
	{ID: 88, Title: "Goroutine Leak Fix", Difficulty: "hard", Category: "concurrency", Points: 210, TimeBonus: 150, Description: "Fix this goroutine leak: a goroutine sends to a channel but nobody reads. Use done channel to signal stop.", Template: "package main\n\nimport (\n\t\"fmt\"\n\t\"time\"\n)\n\nfunc producer(done <-chan struct{}) <-chan int {\n\tch := make(chan int)\n\tgo func() {\n\t\tdefer close(ch)\n\t\tfor i := 0; ; i++ {\n\t\t\tselect {\n\t\t\tcase ch <- i:\n\t\t\tcase <-done:\n\t\t\t\treturn\n\t\t\t}\n\t\t}\n\t}()\n\treturn ch\n}\n\nfunc main() {\n\tdone := make(chan struct{})\n\tch := producer(done)\n\tfor i := 0; i < 5; i++ {\n\t\tfmt.Println(<-ch)\n\t}\n\tclose(done)\n\ttime.Sleep(10 * time.Millisecond)\n\tfmt.Println(\"no leak!\")\n}", Hints: []string{"done channel signals goroutine to stop"}},
	{ID: 89, Title: "Recursive Flatten", Difficulty: "medium", Category: "recursion", Points: 130, TimeBonus: 90, Description: "Use interface{} to flatten deeply nested slices. Flatten [1,[2,[3,4]],5] into [1,2,3,4,5].", Template: "package main\n\nimport \"fmt\"\n\nfunc flatten(val interface{}) []int {\n\t// Your code\n}\n\nfunc main() {\n\tnested := []interface{}{1, []interface{}{2, []interface{}{3, 4}}, 5}\n\tfmt.Println(flatten(nested))\n}", Hints: []string{}},
	{ID: 90, Title: "HTTP Middleware", Difficulty: "hard", Category: "web", Points: 210, TimeBonus: 150, Description: "Write a logging middleware that wraps an http.Handler and prints request method+path. Test with httptest.", Template: "package main\n\nimport (\n\t\"fmt\"\n\t\"net/http\"\n\t\"net/http/httptest\"\n)\n\nfunc loggingMiddleware(next http.Handler) http.Handler {\n\treturn http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {\n\t\tfmt.Printf(\"%s %s\\n\", r.Method, r.URL.Path)\n\t\tnext.ServeHTTP(w, r)\n\t})\n}\n\nfunc main() {\n\thandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {\n\t\tw.Write([]byte(\"ok\"))\n\t})\n\tts := httptest.NewServer(loggingMiddleware(handler))\n\tdefer ts.Close()\n\thttp.Get(ts.URL + \"/hello\")\n}", Hints: []string{"Middleware: func(Handler) Handler pattern"}},
	{ID: 91, Title: "Generics Stack", Difficulty: "hard", Category: "generics", Points: 220, TimeBonus: 160, Description: "Implement a generic Stack[T any] with Push, Pop, Peek methods. Use with both int and string.", Template: "package main\n\nimport \"fmt\"\n\ntype Stack[T any] struct {\n\titems []T\n}\n\nfunc (s *Stack[T]) Push(item T) {\n\t// Your code\n}\n\nfunc (s *Stack[T]) Pop() (T, bool) {\n\t// Your code\n}\n\nfunc main() {\n\ts := &Stack[int]{}\n\ts.Push(1); s.Push(2); s.Push(3)\n\tfor {\n\t\tv, ok := s.Pop()\n\t\tif !ok { break }\n\t\tfmt.Println(v)\n\t}\n}", Hints: []string{"T is the type parameter, works for any type"}},
	{ID: 92, Title: "Pipeline Done", Difficulty: "hard", Category: "concurrency", Points: 230, TimeBonus: 165, Description: "Implement a cancellable pipeline using done channel. Generate squares of 1-10, cancel after receiving 5 results.", Template: "package main\n\nimport \"fmt\"\n\nfunc gen(done <-chan struct{}, nums ...int) <-chan int {\n\tout := make(chan int)\n\tgo func() {\n\t\tdefer close(out)\n\t\tfor _, n := range nums {\n\t\t\tselect {\n\t\t\tcase out <- n:\n\t\t\tcase <-done:\n\t\t\t\treturn\n\t\t\t}\n\t\t}\n\t}()\n\treturn out\n}\n\nfunc sq(done <-chan struct{}, in <-chan int) <-chan int {\n\tout := make(chan int)\n\tgo func() {\n\t\tdefer close(out)\n\t\tfor n := range in {\n\t\t\tselect {\n\t\t\tcase out <- n * n:\n\t\t\tcase <-done:\n\t\t\t\treturn\n\t\t\t}\n\t\t}\n\t}()\n\treturn out\n}\n\nfunc main() {\n\tdone := make(chan struct{})\n\tc := sq(done, gen(done, 1,2,3,4,5,6,7,8,9,10))\n\tfor i := 0; i < 5; i++ {\n\t\tfmt.Println(<-c)\n\t}\n\tclose(done)\n}", Hints: []string{"close(done) propagates cancellation through pipeline"}},
	{ID: 93, Title: "Interfaces Compose", Difficulty: "medium", Category: "interfaces", Points: 140, TimeBonus: 100, Description: "Compose Saver and Loader into a Store interface. Implement with a MemStore. Save and load a value.", Template: "package main\n\nimport \"fmt\"\n\ntype Saver interface { Save(key, val string) }\ntype Loader interface { Load(key string) string }\ntype Store interface { Saver; Loader }\n\ntype MemStore struct { data map[string]string }\n\nfunc (m *MemStore) Save(k, v string) { m.data[k] = v }\nfunc (m *MemStore) Load(k string) string { return m.data[k] }\n\nfunc use(s Store) {\n\ts.Save(\"lang\", \"Go\")\n\tfmt.Println(s.Load(\"lang\"))\n}\n\nfunc main() {\n\tuse(&MemStore{data: map[string]string{}})\n}", Hints: []string{"Interface embedding composes two interfaces into one"}},
	{ID: 94, Title: "Grep Tool", Difficulty: "medium", Category: "strings", Points: 130, TimeBonus: 90, Description: "Write a `grep(pattern, text string) []string` returning lines matching the pattern. Test with a multiline string.", Template: "package main\n\nimport (\n\t\"fmt\"\n\t\"strings\"\n)\n\nfunc grep(pattern, text string) []string {\n\tvar results []string\n\tfor _, line := range strings.Split(text, \"\\n\") {\n\t\t// Your code\n\t}\n\treturn results\n}\n\nfunc main() {\n\ttext := \"go is fast\\npython is slow\\ngo rocks\\njava is verbose\"\n\tfor _, l := range grep(\"go\", text) {\n\t\tfmt.Println(l)\n\t}\n}", Hints: []string{"strings.Contains checks if line has pattern"}},
	{ID: 95, Title: "Chained Errors", Difficulty: "hard", Category: "errors", Points: 195, TimeBonus: 140, Description: "Implement errors.As to extract a specific error type from a wrapped chain. Wrap a *NotFoundError and extract it.", Template: "package main\n\nimport (\n\t\"errors\"\n\t\"fmt\"\n)\n\ntype NotFoundError struct { Resource string }\nfunc (e *NotFoundError) Error() string { return e.Resource + \" not found\" }\n\nfunc getUser(id int) error {\n\treturn fmt.Errorf(\"getUser: %w\", &NotFoundError{\"user\"})\n}\n\nfunc main() {\n\terr := getUser(42)\n\tvar nfe *NotFoundError\n\tif errors.As(err, &nfe) {\n\t\tfmt.Println(\"Resource:\", nfe.Resource)\n\t}\n}", Hints: []string{"errors.As unwraps until it finds target type"}},
	{ID: 96, Title: "String Interning", Difficulty: "medium", Category: "strings", Points: 120, TimeBonus: 80, Description: "Build a string intern pool using a map. Intern 'hello' twice and verify they're the same pointer address.", Template: "package main\n\nimport \"fmt\"\n\nvar pool = map[string]string{}\n\nfunc intern(s string) string {\n\tif v, ok := pool[s]; ok { return v }\n\tpool[s] = s\n\treturn s\n}\n\nfunc main() {\n\ta := intern(\"hello\")\n\tb := intern(\"hello\")\n\tfmt.Println(a == b)\n\tfmt.Printf(\"%p %p\\n\", &a, &b) // different stack vars\n\tfmt.Println(pool)\n}", Hints: []string{"Map lookup returns existing entry"}},
	{ID: 97, Title: "Bitwise Ops", Difficulty: "medium", Category: "algorithms", Points: 125, TimeBonus: 85, Description: "Write functions: `isPowerOfTwo(n int) bool` and `countBits(n int) int`. Test with 16, 7, 255.", Template: "package main\n\nimport \"fmt\"\n\nfunc isPowerOfTwo(n int) bool {\n\t// Your code\n}\n\nfunc countBits(n int) int {\n\t// Your code\n}\n\nfunc main() {\n\tfmt.Println(isPowerOfTwo(16), isPowerOfTwo(7))\n\tfmt.Println(countBits(255))\n}", Hints: []string{"n & (n-1) == 0 for powers of 2", "Brian Kernighan's trick for counting bits"}},
	{ID: 98, Title: "CSV Parser", Difficulty: "medium", Category: "stdlib", Points: 130, TimeBonus: 90, Description: "Parse a CSV string using encoding/csv. Parse 'name,age\\nAlice,30\\nBob,25' and print each record.", Template: "package main\n\nimport (\n\t\"encoding/csv\"\n\t\"fmt\"\n\t\"strings\"\n)\n\nfunc main() {\n\tdata := \"name,age\\nAlice,30\\nBob,25\"\n\tr := csv.NewReader(strings.NewReader(data))\n\t// Read all records and print\n}", Hints: []string{"r.ReadAll() returns [][]string, error"}},
	{ID: 99, Title: "Race Condition", Difficulty: "hard", Category: "concurrency", Points: 240, TimeBonus: 170, Description: "Demonstrate and fix a race condition. Write a bank account with concurrent deposits using sync.Mutex.", Template: "package main\n\nimport (\n\t\"fmt\"\n\t\"sync\"\n)\n\ntype Account struct {\n\tmu      sync.Mutex\n\tbalance int\n}\n\nfunc (a *Account) Deposit(amount int) {\n\ta.mu.Lock()\n\tdefer a.mu.Unlock()\n\ta.balance += amount\n}\n\nfunc main() {\n\tvar wg sync.WaitGroup\n\tacc := &Account{}\n\tfor i := 0; i < 1000; i++ {\n\t\twg.Add(1)\n\t\tgo func() {\n\t\t\tdefer wg.Done()\n\t\t\tacc.Deposit(1)\n\t\t}()\n\t}\n\twg.Wait()\n\tfmt.Println(acc.balance) // Should be 1000\n}", Hints: []string{"Always lock before accessing shared state"}},
	{ID: 100, Title: "Mini ORM Query Builder", Difficulty: "hard", Category: "patterns", Points: 300, TimeBonus: 200, Description: "Build a mini query builder supporting SELECT, WHERE, ORDER BY, LIMIT. Chain them fluently and call .SQL() to get the string.", Template: "package main\n\nimport (\n\t\"fmt\"\n\t\"strings\"\n)\n\ntype QB struct {\n\ttable   string\n\tfields  []string\n\twhere   []string\n\torderBy string\n\tlimit   int\n}\n\nfunc From(table string) *QB { return &QB{table: table} }\n\nfunc (q *QB) Select(fields ...string) *QB {\n\tq.fields = fields\n\treturn q\n}\n\nfunc (q *QB) Where(cond string) *QB {\n\tq.where = append(q.where, cond)\n\treturn q\n}\n\nfunc (q *QB) OrderBy(field string) *QB {\n\tq.orderBy = field\n\treturn q\n}\n\nfunc (q *QB) Limit(n int) *QB {\n\tq.limit = n\n\treturn q\n}\n\nfunc (q *QB) SQL() string {\n\t// Build the SQL string\n\tfields := \"*\"\n\tif len(q.fields) > 0 {\n\t\tfields = strings.Join(q.fields, \", \")\n\t}\n\tsql := fmt.Sprintf(\"SELECT %s FROM %s\", fields, q.table)\n\tif len(q.where) > 0 {\n\t\tsql += \" WHERE \" + strings.Join(q.where, \" AND \")\n\t}\n\tif q.orderBy != \"\" {\n\t\tsql += \" ORDER BY \" + q.orderBy\n\t}\n\tif q.limit > 0 {\n\t\tsql += fmt.Sprintf(\" LIMIT %d\", q.limit)\n\t}\n\treturn sql\n}\n\nfunc main() {\n\tq := From(\"users\").\n\t\tSelect(\"id\", \"name\", \"email\").\n\t\tWhere(\"age > 18\").\n\t\tWhere(\"active = true\").\n\t\tOrderBy(\"name\").\n\t\tLimit(10)\n\tfmt.Println(q.SQL())\n}", Hints: []string{"Build SQL string piece by piece", "Method chaining returns *QB"}},
}

// ─── Achievements Definition ──────────────────────────────────────────────────

var allAchievements = []Achievement{
	// Milestones
	{ID: "first_blood", Name: "First Blood", Description: "Solve your first challenge", Icon: "🩸"},
	{ID: "gopher", Name: "Gopher", Description: "Solve 10 challenges", Icon: "🐹"},
	{ID: "master_gopher", Name: "Master Gopher", Description: "Solve 25 challenges", Icon: "🦫"},
	{ID: "legend", Name: "Legend", Description: "Solve 50 challenges", Icon: "👑"},
	{ID: "centurion", Name: "Centurion", Description: "Solve all 100 challenges", Icon: "🎯"},
	// Points
	{ID: "pts_100", Name: "Rookie", Description: "Earn 100 points", Icon: "🌱"},
	{ID: "pts_500", Name: "Rising Star", Description: "Earn 500 points", Icon: "⭐"},
	{ID: "pts_1000", Name: "Centurion", Description: "Earn 1000 points", Icon: "🏆"},
	{ID: "pts_2500", Name: "Elite", Description: "Earn 2500 points", Icon: "💫"},
	{ID: "pts_5000", Name: "Grand Master", Description: "Earn 5000 points", Icon: "🌟"},
	// Speed
	{ID: "speed_demon", Name: "Speed Demon", Description: "Solve a challenge in under 60s", Icon: "⚡"},
	{ID: "lightning", Name: "Lightning Fingers", Description: "Solve a challenge in under 30s", Icon: "🌩️"},
	{ID: "blink", Name: "Blink", Description: "Solve a challenge in under 10s", Icon: "👁️"},
	// Streaks
	{ID: "on_fire", Name: "On Fire", Description: "5-challenge win streak", Icon: "🔥"},
	{ID: "unstoppable", Name: "Unstoppable", Description: "10-challenge win streak", Icon: "🚀"},
	{ID: "godlike", Name: "Godlike", Description: "25-challenge win streak", Icon: "🌈"},
	// Difficulty
	{ID: "easy_rider", Name: "Easy Rider", Description: "Solve 10 easy challenges", Icon: "🛵"},
	{ID: "middleway", Name: "Middle Way", Description: "Solve 10 medium challenges", Icon: "⚖️"},
	{ID: "perfectionist", Name: "Perfectionist", Description: "Solve your first hard challenge", Icon: "💎"},
	{ID: "hard_boiled", Name: "Hard Boiled", Description: "Solve 10 hard challenges", Icon: "🥚"},
	{ID: "no_easy_way", Name: "No Easy Way", Description: "Solve 5 hard challenges in a row", Icon: "🪨"},
	// Categories
	{ID: "concurrency_king", Name: "Concurrency King", Description: "Solve a concurrency challenge", Icon: "🔀"},
	{ID: "string_wizard", Name: "String Wizard", Description: "Solve 5 string challenges", Icon: "🧵"},
	{ID: "algo_master", Name: "Algo Master", Description: "Solve 5 algorithm challenges", Icon: "🧠"},
	{ID: "data_hoarder", Name: "Data Hoarder", Description: "Solve 5 data-structure challenges", Icon: "🗄️"},
	{ID: "generic_hero", Name: "Generic Hero", Description: "Solve a generics challenge", Icon: "🦸"},
	{ID: "web_dev", Name: "Web Dev", Description: "Solve a web challenge", Icon: "🌐"},
	{ID: "map_maker", Name: "Map Maker", Description: "Solve 3 map challenges", Icon: "🗺️"},
	{ID: "recursion_lord", Name: "Recursion Lord", Description: "Solve 3 recursion challenges", Icon: "🌀"},
	// Time of day
	{ID: "night_owl", Name: "Night Owl", Description: "Code past midnight", Icon: "🦉"},
	{ID: "early_bird", Name: "Early Bird", Description: "Code before 6am", Icon: "🐦"},
	{ID: "lunch_coder", Name: "Lunch Coder", Description: "Solve a challenge between 12–1pm", Icon: "🥪"},
	// Daily
	{ID: "daily_warrior", Name: "Daily Warrior", Description: "Complete a daily challenge", Icon: "📅"},
	{ID: "weekly_grind", Name: "Weekly Grind", Description: "Complete 7 daily challenges", Icon: "📆"},
	// Special
	{ID: "comeback", Name: "Comeback Kid", Description: "Solve a challenge after failing 3×", Icon: "💪"},
	{ID: "hint_free", Name: "Hint Free", Description: "Solve 10 challenges without hints", Icon: "🙈"},
	{ID: "minimalist", Name: "Minimalist", Description: "Solve a challenge with <10 lines", Icon: "✂️"},
	{ID: "polyglot", Name: "Gopher Polyglot", Description: "Use 5 different categories", Icon: "🦜"},
	{ID: "collector", Name: "Collector", Description: "Unlock 10 achievements", Icon: "🎁"},
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func getOrCreatePlayer(id string) *Player {
	mu.Lock()
	defer mu.Unlock()
	if p, ok := players[id]; ok {
		return p
	}
	p := &Player{
		ID:           id,
		Name:         "Gopher",
		Points:       0,
		Level:        1,
		SolvedIDs:    []int{},
		Achievements: []Achievement{},
	}
	players[id] = p
	return p
}

func calcLevel(points int) int {
	if points < 200 {
		return 1
	} else if points < 500 {
		return 2
	} else if points < 1000 {
		return 3
	} else if points < 2000 {
		return 4
	} else if points < 4000 {
		return 5
	}
	return 6 + (points-4000)/2000
}

func contains(ids []int, id int) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

func hasAch(achs []Achievement, id string) bool {
	for _, a := range achs {
		if a.ID == id {
			return true
		}
	}
	return false
}

func checkAchievements(p *Player, q *Question, elapsed int64) []Achievement {
	var unlocked []Achievement

	tryUnlock := func(id string) {
		if hasAch(p.Achievements, id) {
			return
		}
		for _, a := range allAchievements {
			if a.ID == id {
				p.Achievements = append(p.Achievements, a)
				unlocked = append(unlocked, a)
				return
			}
		}
	}

	solved := len(p.SolvedIDs)
	if solved >= 1 {
		tryUnlock("first_blood")
	}
	if elapsed < 60 {
		tryUnlock("speed_demon")
	}
	if elapsed < 30 {
		tryUnlock("lightning")
	}
	if q.Difficulty == "hard" {
		tryUnlock("perfectionist")
	}
	if p.Streak >= 5 {
		tryUnlock("on_fire")
	}
	if p.Points >= 1000 {
		tryUnlock("centurion")
	}
	if solved >= 10 {
		tryUnlock("gopher")
	}
	if solved >= 25 {
		tryUnlock("master_gopher")
	}
	if solved >= 50 {
		tryUnlock("legend")
	}
	if q.ID == dailyQID {
		tryUnlock("daily_warrior")
	}
	if q.Category == "concurrency" {
		tryUnlock("concurrency_king")
	}
	h := time.Now().Hour()
	if h >= 0 && h < 4 {
		tryUnlock("night_owl")
	}

	return unlocked
}

// ─── Handlers ─────────────────────────────────────────────────────────────────

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// GET /api/questions
func handleQuestions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, questions)
}

// GET /api/daily
func handleDaily(w http.ResponseWriter, r *http.Request) {
	for _, q := range questions {
		if q.ID == dailyQID {
			writeJSON(w, q)
			return
		}
	}
	http.NotFound(w, r)
}

// GET /api/player?id=...
func handlePlayer(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	p := getOrCreatePlayer(id)
	mu.RLock()
	defer mu.RUnlock()
	writeJSON(w, p)
}

// POST /api/player/update
func handlePlayerUpdate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	p := getOrCreatePlayer(req.ID)
	mu.Lock()
	p.Name = req.Name
	mu.Unlock()
	writeJSON(w, p)
}

// POST /api/run
func handleRun(w http.ResponseWriter, r *http.Request) {
	var req RunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// Find question
	var q *Question
	for i := range questions {
		if questions[i].ID == req.QuestionID {
			q = &questions[i]
			break
		}
	}
	if q == nil {
		http.Error(w, "question not found", http.StatusNotFound)
		return
	}

	// Write code to temp file and run
	tmpDir, err := os.MkdirTemp("", "gochallenge-*")
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tmpDir)

	codeFile := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(codeFile, []byte(req.Code), 0644); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	cmd := exec.Command("go", "run", codeFile)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Dir = tmpDir

	runErr := cmd.Run()

	resp := RunResponse{}

	if runErr != nil {
		resp.Success = false
		resp.Error = stderr.String()
		if resp.Error == "" {
			resp.Error = runErr.Error()
		}
		writeJSON(w, resp)
		return
	}

	resp.Success = true
	resp.Output = stdout.String()

	// Award points
	p := getOrCreatePlayer(req.PlayerID)
	mu.Lock()
	defer mu.Unlock()

	elapsed := (time.Now().UnixMilli() - req.StartTime) / 1000

	if !contains(p.SolvedIDs, req.QuestionID) {
		p.SolvedIDs = append(p.SolvedIDs, req.QuestionID)
		resp.Points = q.Points

		// Time bonus
		if elapsed < int64(q.TimeBonus) {
			bonus := q.Points / 2
			resp.TimeBonus = bonus
			resp.Points += bonus
		}

		p.Points += resp.Points
		p.Level = calcLevel(p.Points)
		p.Streak++
		p.LastSolved = time.Now()

		resp.Unlocked = checkAchievements(p, q, elapsed)
	}

	writeJSON(w, resp)
}

// POST /api/lint
func handleLint(w http.ResponseWriter, r *http.Request) {
	var req LintRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	tmpDir, err := os.MkdirTemp("", "golint-*")
	if err != nil {
		writeJSON(w, LintResponse{})
		return
	}
	defer os.RemoveAll(tmpDir)

	codeFile := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(codeFile, []byte(req.Code), 0644); err != nil {
		writeJSON(w, LintResponse{})
		return
	}

	// Use go vet for linting
	cmd := exec.Command("go", "vet", codeFile)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Dir = tmpDir
	cmd.Run()

	resp := LintResponse{}
	if out := stderr.String(); out != "" {
		resp.Errors = []string{out}
	}

	writeJSON(w, resp)
}

// GET /api/leaderboard
func handleLeaderboard(w http.ResponseWriter, r *http.Request) {
	mu.RLock()
	defer mu.RUnlock()

	entries := []LeaderboardEntry{}
	for _, p := range players {
		entries = append(entries, LeaderboardEntry{
			ID:     p.ID,
			Name:   p.Name,
			Points: p.Points,
			Level:  p.Level,
			Streak: p.Streak,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Points > entries[j].Points
	})

	for i := range entries {
		entries[i].Rank = i + 1
	}

	writeJSON(w, entries)
}

// ─── Main ─────────────────────────────────────────────────────────────────────

func main() {
	port := "8080"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}

	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/api/questions", corsMiddleware(handleQuestions))
	mux.HandleFunc("/api/daily", corsMiddleware(handleDaily))
	mux.HandleFunc("/api/player", corsMiddleware(handlePlayer))
	mux.HandleFunc("/api/player/update", corsMiddleware(handlePlayerUpdate))
	mux.HandleFunc("/api/run", corsMiddleware(handleRun))
	mux.HandleFunc("/api/lint", corsMiddleware(handleLint))
	mux.HandleFunc("/api/leaderboard", corsMiddleware(handleLeaderboard))

	// Static files
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

	// Root → serve index.html
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./static/index.html")
	})

	fmt.Printf("🐹 GoChallenge server running at http://localhost:%s\n", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}
