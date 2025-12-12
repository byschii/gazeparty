package internal

import (
	"path/filepath"
	"slices"
	"strings"

	"crypto/sha256"
	"fmt"
	"io"
	"os"
)

func extentionMatch(filename string) bool {
	videoExtensions := []string{".mkv", ".mp4", ".avi", ".mov"}
	ext := strings.ToLower(filepath.Ext(filename))
	return slices.Contains(videoExtensions, ext)
}

func IsVideo(filename string) bool {
	extOk := extentionMatch(filename)
	return extOk
}

func SHA256File(filepath string) (string, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func ListVideos(dirPath string) ([]string, error) {
	var videos []string

	// walk dentro la directory
	// crea una lista di tutti i file video trovati
	// il file è indicato con il path completo "{dirPath}/" incluso
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && IsVideo(info.Name()) {

			videos = append(videos, path)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return videos, nil
}

// jaroDistance calcola la distanza Jaro tra due stringhe
func jaroDistance(s1, s2 string) float64 {
	// Se entrambe le stringhe sono vuote, sono identiche
	if len(s1) == 0 && len(s2) == 0 {
		return 1.0
	}

	// Se una delle due è vuota, non c'è similarità
	if len(s1) == 0 || len(s2) == 0 {
		return 0.0
	}

	// Calcola la distanza massima di matching
	matchDistance := max(len(s1), len(s2))/2 - 1
	if matchDistance < 0 {
		matchDistance = 0
	}

	// Array per tracciare i caratteri già matchati
	s1Matches := make([]bool, len(s1))
	s2Matches := make([]bool, len(s2))

	matches := 0
	transpositions := 0

	// Trova i matches
	for i := 0; i < len(s1); i++ {
		start := max(0, i-matchDistance)
		end := min(i+matchDistance+1, len(s2))

		for j := start; j < end; j++ {
			if s2Matches[j] || s1[i] != s2[j] {
				continue
			}
			s1Matches[i] = true
			s2Matches[j] = true
			matches++
			break
		}
	}

	// Se non ci sono matches, la similarità è 0
	if matches == 0 {
		return 0.0
	}

	// Conta le trasposizioni
	k := 0
	for i := 0; i < len(s1); i++ {
		if !s1Matches[i] {
			continue
		}
		for !s2Matches[k] {
			k++
		}
		if s1[i] != s2[k] {
			transpositions++
		}
		k++
	}

	// Calcola la distanza Jaro
	jaro := (float64(matches)/float64(len(s1)) +
		float64(matches)/float64(len(s2)) +
		float64(matches-transpositions/2)/float64(matches)) / 3.0

	return jaro
}

// jaroWinklerDistance calcola la distanza Jaro-Winkler tra due stringhe
// Ritorna un valore tra 0.0 (completamente diverse) e 1.0 (identiche)
func jaroWinklerDistance(s1, s2 string, soglia float64, maxCaratteriPrefix int) float64 {
	jaro := jaroDistance(s1, s2)

	// Se la distanza Jaro è sotto una soglia, non applicare il bonus del prefisso
	if jaro < soglia {
		return jaro
	}

	// Calcola la lunghezza del prefisso comune (max 4 caratteri)
	prefixLength := 0
	maxPrefixLength := min(min(len(s1), len(s2)), maxCaratteriPrefix)

	for i := 0; i < maxPrefixLength; i++ {
		if s1[i] == s2[i] {
			prefixLength++
		} else {
			break
		}
	}

	// Applica il fattore di scaling (p = 0.1 è lo standard)
	p := 0.1
	jaroWinkler := jaro + float64(prefixLength)*p*(1.0-jaro)

	return jaroWinkler
}
