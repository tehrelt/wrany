import { defineConfig } from 'orval'

export default defineConfig({
  wranyApi: {
    input: {
      target: 'http://localhost:8080/swagger/doc.json',
    },
    output: {
      mode: 'tags-split',
      target: 'src/api/generated',
      schemas: 'src/api/generated/model',
      client: 'axios',
      clean: true,
      override: {
        mutator: {
          path: 'src/api/client.ts',
          name: 'customRequest',
        },
        components: {
          schemas: {
            // Strip the Go package prefix from all schema names
            suffix: '',
          },
        },
      },
    },
  },
})
