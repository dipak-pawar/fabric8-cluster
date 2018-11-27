package controller_test

import (
	"testing"

	token "github.com/dgrijalva/jwt-go"
	"github.com/fabric8-services/fabric8-cluster/app/test"
	. "github.com/fabric8-services/fabric8-cluster/controller"
	testsupport "github.com/fabric8-services/fabric8-cluster/test"
	"github.com/fabric8-services/fabric8-common/test/auth"

	"context"
	"github.com/fabric8-services/fabric8-cluster/cluster"
	"github.com/fabric8-services/fabric8-cluster/cluster/repository"
	"github.com/fabric8-services/fabric8-cluster/gormtestsupport"
	"github.com/goadesign/goa"
	"github.com/goadesign/goa/middleware/security/jwt"
	"github.com/jinzhu/gorm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type UserTestSuite struct {
	gormtestsupport.DBTestSuite
}

func (s *UserTestSuite) SetupTest() {
	s.DBTestSuite.SetupTest()
}

func TestRunUserREST(t *testing.T) {
	suite.Run(t, &UserTestSuite{DBTestSuite: gormtestsupport.NewDBTestSuite()})
}

func (s *UserTestSuite) SecuredController(user *auth.Identity) (*goa.Service, *UserController) {
	svc, err := auth.ServiceAsUser("User-Service", user)
	require.NoError(s.T(), err)
	return svc, NewUserController(svc, s.Application)
}
func (s *UserTestSuite) UnsecuredController() (*goa.Service, *UserController) {
	svc := goa.New("User-Service")
	controller := NewUserController(svc, s.Application)
	return svc, controller
}

func (s *UserTestSuite) TestShowClusterAvailableToUser() {

	s.T().Run("ok", func(t *testing.T) {

		t.Run("empty", func(t *testing.T) {
			// given
			identity := auth.NewIdentity()

			// when
			svc, userCtrl := s.SecuredController(identity)
			_, clusters := test.ClustersUserOK(t, svc.Context, svc, userCtrl)

			// then
			assert.NotNil(t, clusters)
			assert.Empty(t, clusters.Data)

		})

		t.Run("list single cluster", func(t *testing.T) {
			// given
			identity := auth.NewIdentity()

			cl := testsupport.CreateCluster(t, s.DB)
			identityCluster := testsupport.CreateIdentityCluster(t, s.DB, cl, &identity.ID)
			require.NotNil(t, identityCluster)

			// when
			svc, userCtrl := s.SecuredController(identity)
			_, clusters := test.ClustersUserOK(t, svc.Context, svc, userCtrl)

			// then
			require.NotNil(t, clusters)
			require.NotNil(t, clusters.Data)

			testsupport.AssertEqualClusterData(t, []repository.Cluster{*cl}, clusters.Data)
		})

		t.Run("list multiple cluster", func(t *testing.T) {
			// given
			expectedClusters := make([]repository.Cluster, 0)
			// create cluster identity for random cluster
			identity := auth.NewIdentity()
			identityID := identity.ID

			c := testsupport.CreateCluster(t, s.DB)
			expectedClusters = append(expectedClusters, *c)

			identityCluster := testsupport.CreateIdentityCluster(t, s.DB, c, &identityID)
			require.NotNil(t, identityCluster)

			// create cluster identity for cluster loading from config
			err := s.Application.ClusterService().CreateOrSaveClusterFromConfig(context.Background())
			require.NoError(s.T(), err)

			osoClusters, err := s.Application.Clusters().Query(func(db *gorm.DB) *gorm.DB {
				return db.Where("type = ?", cluster.OSO)
			})

			for _, c := range osoClusters {
				cl := c
				identityCluster := testsupport.CreateIdentityCluster(t, s.DB, &cl, &identityID)
				require.NotNil(t, identityCluster)
				expectedClusters = append(expectedClusters, cl)
			}

			// when
			svc, userCtrl := s.SecuredController(identity)
			_, clusters := test.ClustersUserOK(t, svc.Context, svc, userCtrl)

			// then
			require.NotNil(t, clusters)
			require.NotNil(t, clusters.Data)

			testsupport.AssertEqualClusterData(t, expectedClusters, clusters.Data)
		})
	})

	s.T().Run("internal error", func(t *testing.T) {

		t.Run("missing token", func(t *testing.T) {
			// given
			svc, userCtrl := s.UnsecuredController()
			// when/then
			test.ClustersUserInternalServerError(t, svc.Context, svc, userCtrl)
		})

		t.Run("ID not a UUID", func(t *testing.T) {
			// given
			jwtToken := token.New(token.SigningMethodRS256)
			jwtToken.Claims.(token.MapClaims)["sub"] = "aa"
			ctx := jwt.WithJWT(context.Background(), jwtToken)
			svc, userCtrl := s.UnsecuredController()
			// when/then
			test.ClustersUserInternalServerError(t, ctx, svc, userCtrl)
		})

		t.Run("token without identity", func(t *testing.T) {
			// given
			jwtToken := token.New(token.SigningMethodRS256)
			ctx := jwt.WithJWT(context.Background(), jwtToken)
			svc, userCtrl := s.UnsecuredController()
			// when/then
			test.ClustersUserInternalServerError(t, ctx, svc, userCtrl)
		})
	})
}
