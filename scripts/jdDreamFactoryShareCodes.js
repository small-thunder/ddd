let codes = ['Bz0ca61N3tL5zO9LnNej_6DrtvFSQkmRwCu_Ko8UAt0=
for (let i = 0; i < codes.length; i++) {
	const index = (i + 1 === 1) ? '' : (i + 1);
	exports['shareCodes.js' + index] = codes[i];
}