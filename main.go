package main

// Importation des bibliothèques nécessaires
import (
	"bufio"         // Pour la lecture des fichiers
	"fmt"           // affichage formaté
	"html/template" // template HTML
	"math/rand"     // choix aléatoire
	"net/http"      // serveur HTTP
	"os"            // opérations sur les fichiers
	"strconv"       // conversion de chaînes
	"strings"       // manipulation de chaînes
	"time"          // gestion du temps
)

// Structure représentant une partie de jeu

type GameState struct {
	Word             string
	MaskedWord       string
	IncorrectGuesses []string
	CorrectGuesses   []string
	AttemptsLeft     int
	GameStatus       string
}

// Liste des mots et sessions de jeu en mémoire

var (
	words    = []string{"fromage", "ordinateur", "avion", "piano", "golang"}
	sessions = map[string]GameState{}
)

// Chargement des mots depuis un fichier

func loadWords(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var w []string
	for scanner.Scan() {
		s := strings.TrimSpace(scanner.Text())
		if s == "" {
			continue
		}
		w = append(w, strings.ToLower(s))
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if len(w) > 0 {
		words = w
	}
	return nil
}

// Choix aléatoire d'un mot

func randomWord() string {
	if len(words) == 0 {
		return "gopher"
	}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return words[r.Intn(len(words))]
}

// Crée une nouvelle partie

func startgame() GameState {
	word := randomWord()
	masked := make([]string, len(word))
	for i := range masked {
		masked[i] = "_"
	}
	return GameState{
		Word:             word,
		MaskedWord:       strings.Join(masked, " "),
		IncorrectGuesses: []string{},
		CorrectGuesses:   []string{},
		AttemptsLeft:     6,
		GameStatus:       "playing",
	}
}

// Vérifie si une lettre a déjà été devinée

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Met à jour le jeu après une lettre devinée

func updateGameState(game *GameState, guess string) {
	guess = strings.ToLower(guess)
	if len(guess) != 1 {
		return
	}
	if contains(game.CorrectGuesses, guess) || contains(game.IncorrectGuesses, guess) {
		return
	}

	found := false
	for _, ch := range game.Word {
		if string(ch) == guess {
			found = true
			break
		}
	}

	if found {
		game.CorrectGuesses = append(game.CorrectGuesses, guess)
		// Met à jour le mot masqué
		parts := make([]string, len(game.Word))
		for i, ch := range game.Word {
			s := string(ch)
			if contains(game.CorrectGuesses, s) {
				parts[i] = s
			} else {
				parts[i] = "_"
			}
		}
		game.MaskedWord = strings.Join(parts, " ")
		if !strings.Contains(game.MaskedWord, "_") {
			game.GameStatus = "won"
		}
	} else {
		game.IncorrectGuesses = append(game.IncorrectGuesses, guess)
		game.AttemptsLeft--
		if game.AttemptsLeft <= 0 {
			game.GameStatus = "lost"
		}
	}
}

// Génère un ID de session unique
func generateSessionID() string {
	return "sid-" + strconv.FormatInt(time.Now().UnixNano(), 10) + "-" + strconv.FormatInt(rand.Int63(), 10)
}

// Page d'accueil

func homeHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("template/home.html"))
	tmpl.Execute(w, nil)
}

// Démarre une nouvelle partie

func newHandler(w http.ResponseWriter, r *http.Request) {
	game := startgame()
	sid := generateSessionID()
	sessions[sid] = game

	http.SetCookie(w, &http.Cookie{
		Name:  "sessionid",
		Value: sid,
		Path:  "/",
	})
	http.Redirect(w, r, "/game", http.StatusSeeOther)
}

// Récupère la partie associée à la session

func getSessionGame(r *http.Request) (string, GameState, bool) {
	c, err := r.Cookie("sessionid")
	if err != nil || c.Value == "" {
		return "", GameState{}, false
	}
	sid := c.Value
	g, ok := sessions[sid]
	return sid, g, ok
}

// Page de jeu principale

func gameHandler(w http.ResponseWriter, r *http.Request) {
	_, game, ok := getSessionGame(r)
	if !ok {
		http.Redirect(w, r, "/new", http.StatusSeeOther)
		return
	}
	if game.GameStatus != "playing" {
		http.Redirect(w, r, "/gameover", http.StatusSeeOther)
		return
	}

	maxWrong := 6
	step := maxWrong - game.AttemptsLeft
	if step < 0 {
		step = 0
	}
	if step > maxWrong {
		step = maxWrong
	}
	// chemin de l'image correspondant au jeu
	imagePath := fmt.Sprintf("/static/images/step%d.svg", step)

	// Préparation de l'alphabet et des lettres devinées

	alphabet := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z"}
	guessed := make(map[string]bool)
	for _, g := range game.CorrectGuesses {
		guessed[g] = true
	}
	for _, g := range game.IncorrectGuesses {
		guessed[g] = true
	}

	tmpl := template.Must(template.ParseFiles("template/index.html"))
	view := struct {
		Game     GameState
		Image    string
		IsOver   bool
		Won      bool
		Lost     bool
		Alphabet []string
		Guessed  map[string]bool
	}{
		Game:     game,
		Image:    imagePath,
		IsOver:   game.GameStatus != "playing",
		Won:      game.GameStatus == "won",
		Lost:     game.GameStatus == "lost",
		Alphabet: alphabet,
		Guessed:  guessed,
	}
	tmpl.Execute(w, view)
}

// Gère les soumissions de lettres devinées

func guessHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/game", http.StatusSeeOther)
		return
	}
	sid, game, ok := getSessionGame(r)
	if !ok {
		http.Redirect(w, r, "/new", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/game", http.StatusSeeOther)
		return
	}
	guess := strings.ToLower(r.FormValue("guess"))
	if game.GameStatus == "playing" && guess != "" {
		updateGameState(&game, guess)
	}
	sessions[sid] = game
	http.Redirect(w, r, "/game", http.StatusSeeOther)
}

// Page de fin de partie

func gameOverHandler(w http.ResponseWriter, r *http.Request) {
	_, game, ok := getSessionGame(r)
	if !ok || game.GameStatus == "playing" {
		http.Redirect(w, r, "/game", http.StatusSeeOther)
		return
	}

	maxWrong := 6
	step := maxWrong - game.AttemptsLeft
	if step < 0 {
		step = 0
	}
	if step > maxWrong {
		step = maxWrong
	}

	imagePath := fmt.Sprintf("/static/images/step%d.svg", step)

	tmpl := template.Must(template.ParseFiles("template/gameover.html"))
	view := struct {
		Game  GameState
		Image string
		Won   bool
		Lost  bool
	}{
		Game:  game,
		Image: imagePath,
		Won:   game.GameStatus == "won",
		Lost:  game.GameStatus == "lost",
	}

	tmpl.Execute(w, view)
}

// Lancement du serveur

func main() {
	// Tente de charger les mots depuis un fichier mots.txt
	if err := loadWords("mots.txt"); err != nil {
		fmt.Println("Warning: could not load mots.txt, using built-in words:", err)
	}

	// Initialisation du générateur de nombres aléatoires
	rand.Seed(time.Now().UnixNano())

	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/new", newHandler)
	http.HandleFunc("/game", gameHandler)
	http.HandleFunc("/guess", guessHandler)
	http.HandleFunc("/gameover", gameOverHandler)

	fmt.Println("Server listening on http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		fmt.Println("Server failed:", err)
	}
}
